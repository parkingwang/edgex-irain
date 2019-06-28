package main

import (
	"encoding/hex"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-socket"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"time"
)

//
// Author: 陈哈哈 bitschen@163.com

const (
	// 设备地址格式：　SWITCH - 控制器地址 - 门号
	formatSwitchAddr = "SWITCH-%d-%d"
)

func main() {
	edgex.Run(endpoint)
}

func endpoint(ctx edgex.Context) error {
	config := ctx.LoadConfig()
	nodeName := value.Of(config["NodeName"]).String()
	rpcAddress := value.Of(config["RpcAddress"]).String()

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	remoteAddress := value.Of(sockOpts["remoteAddress"]).String()

	boardOpts := value.Of(config["BoardOptions"]).MustMap()
	controllerId := uint32(value.Of(boardOpts["controllerId"]).MustInt64())
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(4)

	// AT指令解析
	atRegistry := at.NewAtRegister()
	atCommands(atRegistry, byte(controllerId))

	ctx.Log().Debugf("TCP客户端连接: [%s]", remoteAddress)

	cli := sock.New(sock.Options{
		Network:           "tcp",
		Addr:              remoteAddress,
		ReadTimeout:       value.Of(sockOpts["readTimeout"]).DurationOfDefault(time.Second),
		WriteTimeout:      value.Of(sockOpts["writeTimeout"]).DurationOfDefault(time.Second),
		KeepAlive:         value.Of(sockOpts["keepAlive"]).BoolOrDefault(true),
		KeepAliveInterval: value.Of(sockOpts["keepAliveInterval"]).DurationOfDefault(time.Second * 10),
		ReconnectDelay:    value.Of(sockOpts["reconnectDelay"]).DurationOfDefault(time.Second),
	})
	if err := cli.Connect(); nil != err {
		ctx.Log().Error("TCP客户端连接失败", err)
	} else {
		ctx.Log().Debug("TCP客户端连接成功")
	}
	defer func() {
		if err := cli.Disconnect(); nil != err {
			ctx.Log().Error("TCP客户端关闭连接失败：", err)
		}
	}()

	buffer := make([]byte, 256)
	endpoint := ctx.NewEndpoint(edgex.EndpointOptions{
		NodeName:        nodeName,
		RpcAddr:         rpcAddress,
		SerialExecuting: true, // 艾润品牌主板不支持并发处理
		InspectFunc:     inspectFunc(controllerId, int(doorCount)),
	})

	// 处理控制指令
	endpoint.Serve(func(msg edgex.Message) (out edgex.Message) {
		atCmd := string(msg.Body())
		ctx.Log().Debug("接收到控制指令: " + atCmd)
		cmd, err := atRegistry.Apply(atCmd)
		if nil != err {
			return edgex.NewMessageString(nodeName, "EX=ERR:"+err.Error())
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("艾润指令码: " + hex.EncodeToString(cmd))
		})
		// Write
		if _, err := cli.Write(cmd); nil != err {
			return edgex.NewMessageString(nodeName, "EX=ERR:"+err.Error())
		}
		// Read
		var n = int(0)
		for i := 0; i < 5; i++ {
			if n, err = cli.Read(buffer); nil != err {
				ctx.Log().Errorf("读取设备响应数据出错[%d]: %s", i, err.Error())
				<-time.After(time.Millisecond * 500)
			} else {
				break
			}
		}
		// parse
		body := "EX=ERR:NO-REPLY"
		data := buffer[:n]
		if n > 0 {
			if irain.CheckProtoValid(data) {
				ctx.Log().Error("解析响应数据出错", err)
				body = "EX=ERR:PARSE_ERR"
			} else {
				body = "EX=OK"
			}
		}
		ctx.Log().Debug("接收到控制响应: " + body)
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("响应码: " + hex.EncodeToString(data))
		})
		return edgex.NewMessageString(nodeName, body)
	})

	endpoint.Startup()
	defer endpoint.Shutdown()

	return ctx.TermAwait()
}

func inspectFunc(controllerId uint32, doorCount int) func() edgex.Inspect {
	deviceOf := func(doorId int) edgex.VirtualNode {
		// Address 可以自动从环境变量中获取
		return edgex.VirtualNode{
			VirtualNodeName: fmt.Sprintf(formatSwitchAddr, controllerId, doorId),
			Desc:            fmt.Sprintf("%d号门-电磁开关", doorId),
			Type:            edgex.NodeTypeEndpoint,
			Virtual:         true,
			Command:         fmt.Sprintf("AT+OPEN=%d", doorId),
		}
	}
	return func() edgex.Inspect {
		nodes := make([]edgex.VirtualNode, doorCount)
		for d := 0; d < doorCount; d++ {
			nodes[d] = deviceOf(d + 1)
		}
		return edgex.Inspect{
			Vendor:       irain.VendorName,
			DriverName:   irain.DriverName,
			VirtualNodes: nodes,
		}
	}
}
