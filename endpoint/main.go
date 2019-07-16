package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-socket"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"io"
	"time"
)

//
// Author: 陈哈哈 bitschen@163.com

var (
	RepOK   = errors.New("EX=OK")
	RepFail = errors.New("EX=ERR:FAILED")
	RepNop  = errors.New("EX=ERR:NO_VALID_REPLY")
)

func main() {
	edgex.Run(endpoint)
}

func endpoint(ctx edgex.Context) error {
	config := ctx.LoadConfig()
	nodeName := value.Of(config["NodeName"]).String()
	rpcAddress := value.Of(config["RpcAddress"]).String()

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	network := value.Of(sockOpts["network"]).StringOrDefault("tcp")
	remoteAddress := value.Of(sockOpts["remoteAddress"]).String()

	boardOpts := value.Of(config["BoardOptions"]).MustMap()
	controllerId := uint32(value.Of(boardOpts["controllerId"]).MustInt64())
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(4)

	// AT指令解析
	atRegistry := at.NewAtRegister()
	atCommands(atRegistry, byte(controllerId))

	log := ctx.Log()

	cli := sock.New(sock.Options{
		Network:           network,
		Addr:              remoteAddress,
		ReadTimeout:       value.Of(sockOpts["readTimeout"]).DurationOfDefault(time.Second),
		WriteTimeout:      value.Of(sockOpts["writeTimeout"]).DurationOfDefault(time.Second),
		KeepAlive:         value.Of(sockOpts["keepAlive"]).BoolOrDefault(true),
		KeepAliveInterval: value.Of(sockOpts["keepAliveInterval"]).DurationOfDefault(time.Second * 10),
		ReconnectDelay:    value.Of(sockOpts["reconnectDelay"]).DurationOfDefault(time.Second),
	})

	log.Debugf("客户端连接: [%s] %s", network, remoteAddress)

	if err := cli.Connect(); nil != err {
		log.Error("客户端连接失败", err)
	} else {
		log.Debug("客户端连接成功")
	}
	defer func() {
		if err := cli.Disconnect(); nil != err {
			log.Error("客户端关闭连接失败：", err)
		}
	}()

	endpoint := ctx.NewEndpoint(edgex.EndpointOptions{
		NodeName:        nodeName,
		RpcAddr:         rpcAddress,
		SerialExecuting: true, // 艾润品牌主板不支持并发处理
		InspectNodeFunc: nodeFunc(nodeName, controllerId, int(doorCount)),
	})

	// 处理控制指令
	endpoint.Serve(func(msg edgex.Message) (out edgex.Message) {
		atCmd := string(msg.Body())
		log.Debug("接收到控制指令: " + atCmd)
		cmd, err := atRegistry.Apply(atCmd)
		if nil != err {
			log.Error("控制指令格式错误: " + err.Error())
			return endpoint.NextMessage(nodeName, []byte("EX=ERR:BAD_CMD:"+err.Error()))
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("艾润控制指令码: " + hex.EncodeToString(cmd))
		})
		// Write
		if _, err := cli.Write(cmd); nil != err {
			log.Error("发送/写入控制指令出错", err)
			return endpoint.NextMessage(nodeName, []byte("EX=ERR:WRITE:"+err.Error()))
		}
		reply := tryReadReply(ctx, cli)
		return endpoint.NextMessage(nodeName, []byte(reply))
	})

	endpoint.Startup()
	defer endpoint.Shutdown()

	return ctx.TermAwait()
}

// 读取设备响应数据
// 只读取应答指令，忽略其它指令。最多读取5次，间隔100毫秒
func tryReadReply(ctx edgex.Context, cli *sock.Client) string {
	log := ctx.Log()
	msg := new(irain.Message)
	for i := 0; i < 5; i++ {
		err := cli.ReadWith(func(in io.Reader) error {
			if ok, e := irain.ReadMessage(in, msg); !ok {
				return e
			} else if msg.IsSuccess() {
				return RepOK
			} else {
				return RepFail
			}
		})
		if err != RepOK && err != RepFail {
			log.Errorf("读取设备响应数据出错[%d]: %s", i, err.Error())
			<-time.After(time.Millisecond * 100)
			continue
		} else {
			return err.Error()
		}
	}
	return RepNop.Error()
}

func nodeFunc(nodeName string, controllerId uint32, doorCount int) func() edgex.MainNode {
	deviceOf := func(doorId int) edgex.VirtualNode {
		return edgex.VirtualNode{
			Major:      fmt.Sprintf("%d", controllerId),
			Minor:      fmt.Sprintf("%d", doorId),
			Desc:       fmt.Sprintf("%d号门-电磁开关", doorId),
			Virtual:    true,
			RpcCommand: fmt.Sprintf("AT+OPEN=%d", doorId),
		}
	}
	return func() edgex.MainNode {
		nodes := make([]edgex.VirtualNode, doorCount)
		for d := 0; d < doorCount; d++ {
			nodes[d] = deviceOf(d + 1)
		}
		return edgex.MainNode{
			NodeType:     edgex.NodeTypeEndpoint,
			NodeName:     nodeName,
			Vendor:       irain.VendorName,
			ConnDriver:   irain.DriverName,
			VirtualNodes: nodes,
		}
	}
}
