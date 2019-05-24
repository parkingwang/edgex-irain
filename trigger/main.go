package main

import (
	"fmt"
	irain "github.com/nextabc-lab/edgex-dongkong"
	"github.com/nextabc-lab/edgex-go"
	"github.com/yoojia/go-value"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

const (
	// 设备地址格式：　TRIGGER-BID-DOOR_ID-DIRECT
	deviceAddr = "TRIGGER-%d-%d-%d"
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
	monitorPeriod := value.Of(boardOpts["monitorPeriod"]).DurationOfDefault(time.Second)

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	targetAddress := value.Of(sockOpts["targetAddress"]).String()

	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		Name:  triggerName,
		Topic: eventTopic,
	})

	client := irain.NewClientWithOptions(targetAddress, sockOpts)
	if err := client.Open(); nil != err {
		ctx.Log().Error("客户端打开连接失败", err)
	}
	defer func() {
		if err := client.Close(); nil != err {
			ctx.Log().Error("客户端关闭连接失败", err)
		}
	}()

	ticker := time.NewTicker(monitorPeriod)
	defer ticker.Stop()

	// 启用Trigger服务
	trigger.Startup()
	defer trigger.Shutdown()

	scan := irain.NewCommand(controllerId, irain.FunMonitorScan, []byte{}).Bytes()
	buff := make([]byte, client.BufferSize())

	monitor := func() {
		if _, err := client.Send(scan); nil != err {
			ctx.Log().Error("发送监控扫描指令出错", err)
			return
		}
		// 等待响应结果
		event := Event{}
		for retry := 0; retry < 5; retry++ {
			if n, err := client.Receive(buff); nil != err {
				ctx.Log().Error("发送监控扫描指令出错", err)
				<-time.After(time.Millisecond * 100)
			} else {
				if event, err = parseEvent(buff[:n]); nil != err {
					ctx.Log().Error("监控返回无法解析数据")
					return
				}
			}
		}
		// 发送事件
		deviceName := fmt.Sprintf(deviceAddr, event.BoardId, event.Doors, event.Direct)
		if err := trigger.SendEventMessage(edgex.NewMessage([]byte(deviceName), event.Bytes()));
			nil != err {
			ctx.Log().Error("触发事件出错: ", err)
		} else {
			ctx.Log().Debug("触发刷卡事件: ", err)
		}
	}

	for {
		select {
		case <-ctx.TermChan():
			return nil

		case <-ticker.C:
			monitor()
		}
	}
}
