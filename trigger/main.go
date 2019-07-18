package main

import (
	"encoding/hex"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-socket"
	"github.com/yoojia/go-value"
	"go.uber.org/zap"
	"io"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
// 使用Socket客户端连接的Trigger。注意与Endpoint都是使用Client模式。

const (
	nodeIdFormat = "READER:%d:%d:%s"
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
	network := value.Of(sockOpts["network"]).StringOrDefault("tcp")

	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		NodeName:        nodeName,
		Topic:           eventTopic,
		InspectNodeFunc: nodeFunc(nodeName, controllerId, int(doorCount)),
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

	// 等待刷卡数据
	process := func(msg *irain.Message) {
		if FrameCardEventLength != len(msg.Payload) {
			return
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("接收监控事件数据: " + hex.EncodeToString(msg.Payload))
		})
		event := new(Event)
		parseCardEvent(controllerId, msg.Payload, event)
		// 发送事件
		virtualNodeId := fmt.Sprintf(nodeIdFormat, event.ControllerId, event.DoorId, irain.DirectName(event.Direct))
		if err := trigger.SendEventMessage(virtualNodeId, event.Bytes()); nil != err {
			log.Error("触发事件出错: ", err)
		} else {
			log.Debugf("接收到刷卡数据, Device: %s, DoorId: %d, Card[WG26SN]: %s, Card[SN]: %s",
				virtualNodeId, event.DoorId, event.Card.Wg26SN, event.Card.CardSN)
		}
	}

	// 读数据循环
	message := new(irain.Message)
	for {
		select {
		case <-ctx.TermChan():
			return nil

		default:
			err := cli.ReadWith(func(in io.Reader) error {
				if ok, err := irain.ReadMessage(in, message); ok {
					process(message)
					return nil
				} else {
					return err
				}
			})
			if nil != err && !sock.IsNetTempErr(err) {
				ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
					log.Error("读取监控数据出错: " + err.Error())
				})
			}
		}
	}
}

func nodeFunc(nodeName string, controllerId byte, doorCount int) func() edgex.MainNode {
	deviceOf := func(doorId, direct int) edgex.VirtualNode {
		directName := irain.DirectName(byte(direct))
		return edgex.VirtualNode{
			NodeId:  fmt.Sprintf(nodeIdFormat, controllerId, doorId, directName),
			Major:   fmt.Sprintf("%d:%d", controllerId, doorId),
			Minor:   directName,
			Desc:    fmt.Sprintf("%d号门-%s-读卡器", doorId, directName),
			Virtual: true,
		}
	}
	return func() edgex.MainNode {
		nodes := make([]edgex.VirtualNode, doorCount*2)
		for d := 0; d < doorCount; d++ {
			nodes[d*2] = deviceOf(d+1, irain.DirectIn)
			nodes[d*2+1] = deviceOf(d+1, irain.DirectOut)
		}
		return edgex.MainNode{
			NodeType:     edgex.NodeTypeTrigger,
			NodeName:     nodeName,
			Vendor:       irain.VendorName,
			ConnDriver:   irain.DriverName,
			VirtualNodes: nodes,
		}
	}
}
