package main

import (
	"encoding/hex"
	"fmt"
	"github.com/bitschen/go-socket"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"runtime"
	"time"
)

//
// Author: 陈哈哈 bitschen@163.com

func main() {
	edgex.Run(endpoint)
}

func endpoint(ctx edgex.Context) error {
	config := ctx.LoadConfig()
	deviceName := value.Of(config["Name"]).String()
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
		Name:        deviceName,
		RpcAddr:     rpcAddress,
		InspectFunc: inspectFunc(controllerId, int(doorCount)),
	})

	// 处理控制指令
	endpoint.Serve(func(msg edgex.Message) (out edgex.Message) {
		atCmd := string(msg.Body())
		ctx.Log().Debug("接收到控制指令: " + atCmd)
		cmd, err := atRegistry.Apply(atCmd)
		if nil != err {
			return edgex.NewMessageString(deviceName, "EX=ERR:"+err.Error())
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("艾润指令码: " + hex.EncodeToString(cmd))
		})
		// Write
		if _, err := cli.Write(cmd); nil != err {
			return edgex.NewMessageString(deviceName, "EX=ERR:"+err.Error())
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
		return edgex.NewMessageString(deviceName, body)
	})

	endpoint.Startup()
	defer endpoint.Shutdown()

	return ctx.TermAwait()
}

func inspectFunc(sn uint32, doorCount int) func() edgex.Inspect {
	deviceOf := func(doorId int) edgex.Device {
		// Address 可以自动从环境变量中获取
		return edgex.Device{
			Name:    fmt.Sprintf("ENDPOINT-%d-%d", sn, doorId),
			Desc:    fmt.Sprintf("%d号门-控制开关", doorId),
			Type:    edgex.DeviceTypeEndpoint,
			Virtual: true,
			Command: fmt.Sprintf("AT+OPEN=%d", doorId),
		}
	}
	return func() edgex.Inspect {
		devices := make([]edgex.Device, doorCount)
		for d := 0; d < doorCount; d++ {
			devices[d] = deviceOf(d + 1)
		}
		return edgex.Inspect{
			HostOS:     runtime.GOOS,
			HostArch:   runtime.GOARCH,
			Vendor:     irain.VendorName,
			DriverName: irain.DriverName,
			Devices:    devices,
		}
	}
}
