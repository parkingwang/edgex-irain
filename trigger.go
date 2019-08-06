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

const (
	virtualIdFormat = "READER-%d-%d-%s"
)

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
		virtualNodeId := fmt.Sprintf(virtualIdFormat, event.ControllerId, event.DoorId, DirectName(event.Direct))
		if err := trigger.PublishEvent(virtualNodeId, event.Bytes()); nil != err {
			log.Error("触发事件出错: ", err)
		} else {
			log.Debugf("接收到刷卡数据, Device: %s, DoorId: %d, Card[WG26SN]: %s, Card[SN]: %s",
				virtualNodeId, event.DoorId, event.Card.Wg26SN, event.Card.CardSN)
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
func FuncTriggerNode(controllerId byte, doorCount int) func() edgex.MainNodeInfo {
	deviceOf := func(doorId, direct int) *edgex.VirtualNodeInfo {
		directName := DirectName(byte(direct))
		return &edgex.VirtualNodeInfo{
			VirtualId: fmt.Sprintf(virtualIdFormat, controllerId, doorId, directName),
			MajorId:   fmt.Sprintf("%d-%d", controllerId, doorId),
			MinorId:   directName,
			Desc:      fmt.Sprintf("%d号门-%s-读卡器", doorId, directName),
			Virtual:   true,
		}
	}
	return func() edgex.MainNodeInfo {
		nodes := make([]*edgex.VirtualNodeInfo, doorCount*2)
		for d := 0; d < doorCount; d++ {
			nodes[d*2] = deviceOf(d+1, DirectIn)
			nodes[d*2+1] = deviceOf(d+1, DirectOut)
		}
		return edgex.MainNodeInfo{
			NodeType:     edgex.NodeTypeTrigger,
			Vendor:       VendorName,
			ConnDriver:   DriverName,
			VirtualNodes: nodes,
		}
	}
}
