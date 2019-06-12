package main

import (
	"encoding/hex"
	"fmt"
	"github.com/nextabc-lab/edgex-dongkong"
	"github.com/nextabc-lab/edgex-go"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"runtime"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

const (
	// 设备地址格式：　TRIGGER-BID-DOOR_ID-DIRECT
	deviceAddr = "TRIGGER-%d-%d-%s"
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
	scanPeriod := value.Of(boardOpts["scanPeriod"]).DurationOfDefault(time.Millisecond * 500)
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(4)

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	remoteAddress := value.Of(sockOpts["remoteAddress"]).String()

	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		Name:        triggerName,
		Topic:       eventTopic,
		InspectFunc: inspectFunc(controllerId, int(doorCount), eventTopic),
	})

	ctx.Log().Debugf("连接目标地址: [%s]", remoteAddress)
	client := irain.NewClientWithOptions(remoteAddress, sockOpts)
	if err := client.Open(); nil != err {
		ctx.Log().Error("客户端打开连接失败", err)
	} else {
		ctx.Log().Debug("客户端连接成功")
	}
	defer func() {
		if err := client.Close(); nil != err {
			ctx.Log().Error("客户端关闭连接失败：", err)
		}
	}()

	trigger.Startup()
	defer trigger.Shutdown()

	scan := newCommandEventScan(controllerId).Bytes()
	buff := make([]byte, client.BufferSize())

	monitor := func() {
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("发送事件监控扫描指令: " + hex.EncodeToString(scan))
		})
		if _, err := client.Send(scan); nil != err {
			ctx.Log().Error("发送事件监控扫描指令出错: ", err)
			return
		}
		// 等待响应结果
		event := Event{}
		for retry := 0; retry < 5; retry++ {
			if n, err := client.Receive(buff); nil != err {
				ctx.Log().Error("接收事件监控扫描响应出错: ", err)
				<-time.After(time.Millisecond * 200)
			} else {
				if event, err = parseEvent(buff[:n]); nil != err {
					ctx.Log().Error("事件监控返回无法解析数据: ", err)
					return
				}
			}
		}
		// 发送事件
		deviceName := fmt.Sprintf(deviceAddr, event.BoardId, event.Doors, irain.DirectName(event.Direct))
		msg := edgex.NewMessage([]byte(deviceName), event.Bytes())
		if err := trigger.SendEventMessage(msg); nil != err {
			ctx.Log().Error("触发事件出错: ", err)
		} else {
			ctx.Log().Debug("触发刷卡事件: ", err)
		}
	}

	ticker := time.NewTicker(scanPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.TermChan():
			return nil

		case <-ticker.C:
			monitor()
		}
	}
}

func inspectFunc(devAddr byte, doorCount int, eventTopic string) func() edgex.Inspect {
	deviceOf := func(doorId, direct int) edgex.Device {
		directName := irain.DirectName(byte(direct))
		return edgex.Device{
			Name:       fmt.Sprintf(deviceAddr, devAddr, doorId, directName),
			Desc:       fmt.Sprintf("%d号门-%s-读卡器", doorId, directName),
			Type:       edgex.DeviceTypeTrigger,
			Virtual:    true,
			EventTopic: eventTopic,
		}
	}
	return func() edgex.Inspect {
		devices := make([]edgex.Device, doorCount*2)
		for d := 0; d < doorCount; d++ {
			devices[d*2] = deviceOf(d+1, irain.DirectIn)
			devices[d*2+1] = deviceOf(d+1, irain.DirectOut)
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

// 创建事件监控扫描指令
func newCommandEventScan(devAddr byte) *irain.Command {
	return irain.NewCommand(devAddr, irain.CmdEventScan, []byte{})
}
