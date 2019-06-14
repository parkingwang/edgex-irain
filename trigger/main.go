package main

import (
	"encoding/hex"
	"fmt"
	"github.com/bitschen/go-socket"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"runtime"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
// 使用SocketTCP客户端连接的Trigger。注意与Endpoint都是使用Client模式。

const (
	// 设备地址格式：　READER - 控制器地址 - 门号
	formatReaderAddr = "READER-%d-%d"
)

func main() {
	edgex.Run(trigger)
}

func trigger(ctx edgex.Context) error {
	config := ctx.LoadConfig()
	triggerName := value.Of(config["Name"]).String()
	eventTopic := value.Of(config["Topic"]).String()

	boardOpts := value.Of(config["BoardOptions"]).MustMap()
	controllerId := byte(value.Of(boardOpts["controllerId"]).MustInt64())
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(4)

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	remoteAddress := value.Of(sockOpts["remoteAddress"]).String()

	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		Name:        triggerName,
		Topic:       eventTopic,
		InspectFunc: inspectFunc(controllerId, int(doorCount), eventTopic),
	})

	ctx.Log().Debugf("TCP连接服务端地址: [tcp://%s]", remoteAddress)

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

	trigger.Startup()
	defer trigger.Shutdown()

	buffer := make([]byte, 256)

	// 等待刷卡数据
	process := func() {
		n, err := cli.Read(buffer)
		if nil != err {
			if sock.IsNetTempErr(err) {
				return
			}
			ctx.Log().Error("接收监控事件出错: ", err)
			if err := cli.Reconnect(); nil != err {
				ctx.Log().Error("尝试重新连接: ", err)
			}
		}
		data := buffer[:n]
		// 检查艾润的数据格式
		if !irain.CheckProtoValid(data) {
			return
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("接收监控事件数据: " + hex.EncodeToString(data))
		})
		event, err := parseEvent(controllerId, data)
		if nil != err {
			ctx.Log().Error("事件监控返回无法解析数据: ", err)
			return
		}
		// 发送事件
		deviceName := fmt.Sprintf(formatReaderAddr, event.ControllerId, event.DoorId)
		msg := edgex.NewMessage([]byte(deviceName), event.Bytes())
		if err := trigger.SendEventMessage(msg); nil != err {
			ctx.Log().Error("触发事件出错: ", err)
		} else {
			ctx.Log().Debugf("接收到刷卡数据, Device: %s, DoorId: %d, Card[WG26SN]: %s, Card[SN]: %s", deviceName, event.DoorId, event.Card.Wg26SN, event.Card.CardSN)
		}
	}

	for {
		select {
		case <-ctx.TermChan():
			return nil

		default:
			process()
		}
	}
}

func inspectFunc(devAddr byte, doorCount int, eventTopic string) func() edgex.Inspect {
	deviceOf := func(doorId, direct int) edgex.VirtualDevice {
		return edgex.VirtualDevice{
			Name:       fmt.Sprintf(formatReaderAddr, devAddr, doorId),
			Desc:       fmt.Sprintf("%d号门-读卡器", doorId),
			Type:       edgex.DeviceTypeTrigger,
			Virtual:    true,
			EventTopic: eventTopic,
		}
	}
	return func() edgex.Inspect {
		devices := make([]edgex.VirtualDevice, doorCount*2)
		for d := 0; d < doorCount; d++ {
			devices[d*2] = deviceOf(d+1, irain.DirectIn)
			devices[d*2+1] = deviceOf(d+1, irain.DirectOut)
		}
		return edgex.Inspect{
			HostOS:         runtime.GOOS,
			HostArch:       runtime.GOARCH,
			Vendor:         irain.VendorName,
			DriverName:     irain.DriverName,
			VirtualDevices: devices,
		}
	}
}
