package main

import (
	"encoding/hex"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-socket"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
// 使用Socket客户端连接的Trigger。注意与Endpoint都是使用Client模式。

const (
	// 设备地址格式：　READER - 控制器地址 - 门号
	formatReaderAddr = "READER-%d-%d"
)

func main() {
	edgex.Run(trigger)
}

func trigger(ctx edgex.Context) error {
	config := ctx.LoadConfig()
	nodeName := value.Of(config["NodeName"]).String()
	eventTopic := value.Of(config["Topic"]).String()

	boardOpts := value.Of(config["BoardOptions"]).MustMap()
	controllerId := byte(value.Of(boardOpts["controllerId"]).MustInt64())
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(4)

	sockOpts := value.Of(config["SocketClientOptions"]).MustMap()
	remoteAddress := value.Of(sockOpts["remoteAddress"]).String()
	network := value.Of(sockOpts["network"]).StringOrDefault("udp")

	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		NodeName:    nodeName,
		Topic:       eventTopic,
		InspectFunc: inspectFunc(controllerId, int(doorCount), eventTopic),
	})

	cli := sock.New(sock.Options{
		Network:           network,
		Addr:              remoteAddress,
		ReadTimeout:       value.Of(sockOpts["readTimeout"]).DurationOfDefault(time.Second),
		WriteTimeout:      value.Of(sockOpts["writeTimeout"]).DurationOfDefault(time.Second),
		KeepAlive:         value.Of(sockOpts["keepAlive"]).BoolOrDefault(true),
		KeepAliveInterval: value.Of(sockOpts["keepAliveInterval"]).DurationOfDefault(time.Second * 10),
		ReconnectDelay:    value.Of(sockOpts["reconnectDelay"]).DurationOfDefault(time.Second),
	})

	log := ctx.Log()

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
			log.Error("接收监控事件出错: ", err)
			if err := cli.Reconnect(); nil != err {
				log.Error("尝试重新连接: ", err)
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
			log.Error("事件监控返回无法解析数据: ", err)
			return
		}
		// 发送事件
		deviceName := fmt.Sprintf(formatReaderAddr, event.ControllerId, event.DoorId)
		if err := trigger.SendEventMessage(deviceName, event.Bytes()); nil != err {
			log.Error("触发事件出错: ", err)
		} else {
			log.Debugf("接收到刷卡数据, Device: %s, DoorId: %d, Card[WG26SN]: %s, Card[SN]: %s", deviceName, event.DoorId, event.Card.Wg26SN, event.Card.CardSN)
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

func inspectFunc(controllerId byte, doorCount int, eventTopic string) func() edgex.Inspect {
	deviceOf := func(doorId, direct int) edgex.VirtualNode {
		return edgex.VirtualNode{
			VirtualNodeName: fmt.Sprintf(formatReaderAddr, controllerId, doorId),
			Desc:            fmt.Sprintf("%d号门-读卡器", doorId),
			Type:            edgex.NodeTypeTrigger,
			Virtual:         true,
			EventTopic:      eventTopic,
		}
	}
	return func() edgex.Inspect {
		nodes := make([]edgex.VirtualNode, doorCount*2)
		for d := 0; d < doorCount; d++ {
			nodes[d*2] = deviceOf(d+1, irain.DirectIn)
			nodes[d*2+1] = deviceOf(d+1, irain.DirectOut)
		}
		return edgex.Inspect{
			Vendor:       irain.VendorName,
			DriverName:   irain.DriverName,
			VirtualNodes: nodes,
		}
	}
}
