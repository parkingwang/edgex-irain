package irain

import (
	"encoding/hex"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	sock "github.com/yoojia/go-socket"
	"go.uber.org/zap"
	"io"
	"os"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

// 等待刷卡数据循环
func ReceiveLoop(ctx edgex.Context, trigger edgex.Trigger, controllerId byte, cli *sock.Client, shutdown <-chan os.Signal) error {
	log := ctx.Log()
	process := func(msg *Message) {
		if FrameCardEventLength != len(msg.Payload) {
			return
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("接收监控事件数据: " + hex.EncodeToString(msg.Payload))
		})
		event := new(CardEvent)
		ParseCardEvent(controllerId, msg.Payload, event)
		// 发送事件
		ctrlId := makeGroupId(event.ControllerId)
		doorId := makeDoorId(int(event.DoorId))
		directName := DirectName(event.Direct)
		if err := trigger.PublishEvent(
			ctrlId,
			doorId,
			directName,
			event.Bytes(),
			trigger.GenerateEventId()); nil != err {
			log.Error("触发事件出错: ", err)
		} else {
			log.Debugf("接收到刷卡数据, CtrlId: %s, DoorId: %s, Card[WG26SN]: %s, Card[SN]: %s",
				ctrlId, doorId, event.Card.Wg26SN, event.Card.CardSN)
		}
	}

	// 读数据循环
	message := new(Message)
	for {
		select {
		case <-shutdown:
			log.Debug("接收到系统终止信号")
			return nil

		default:
			err := cli.ReadWith(func(in io.Reader) error {
				ok, err := ReadMessage(in, message)
				if ok {
					process(message)
					return nil
				}
				// 过滤格式问题
				if err == ErrUnknownMessage {
					log.Debug("接收到无效监控数据")
					return nil
				} else {
					return err
				}
			})
			if nil != err && !sock.IsNetTempErr(err) {
				ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
					log.Error("读取监控数据出错: " + err.Error())
				})
				log.Debug("正在重新连接")
				if err := cli.Reconnect(); nil != err {
					log.Error("重连失败: ", err)
				}
			}
		}
	}
}

// 创建TriggerNode消息函数
func FuncTriggerProperties(controllerId byte, doorCount int) func() edgex.MainNodeProperties {
	deviceOf := func(doorId, direct int) *edgex.VirtualNodeProperties {
		directName := DirectName(byte(direct))
		return &edgex.VirtualNodeProperties{
			GroupId:     makeGroupId(controllerId),
			MajorId:     makeDoorId(doorId),
			MinorId:     directName,
			Description: fmt.Sprintf("控制器#%d-%d号门-%s-读卡器", controllerId, doorId, directName),
			Virtual:     true,
		}
	}
	return func() edgex.MainNodeProperties {
		nodes := make([]*edgex.VirtualNodeProperties, doorCount*2)
		for d := 0; d < doorCount; d++ {
			nodes[d*2] = deviceOf(d+1, DirectIn)
			nodes[d*2+1] = deviceOf(d+1, DirectOut)
		}
		return edgex.MainNodeProperties{
			NodeType:     edgex.NodeTypeTrigger,
			Vendor:       VendorName,
			ConnDriver:   DriverName,
			VirtualNodes: nodes,
		}
	}
}
