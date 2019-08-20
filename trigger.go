package irain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-go/extra"
	"github.com/parkingwang/go-wg26"
	sock "github.com/yoojia/go-socket"
	"go.uber.org/zap"
	"io"
	"os"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

const FrameCardEventLength = 10

// 等待刷卡数据循环
func ReceiveLoop(ctx edgex.Context, trigger edgex.Trigger, boardAddr byte, cli *sock.Client, shutdown <-chan os.Signal) error {
	log := ctx.Log()
	process := func(msg *IrMessage) {
		if FrameCardEventLength != len(msg.Payload) {
			log.Debug("只处理刷卡类型事件，忽略")
			return
		}

		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("艾润发送事件码: " + hex.EncodeToString(msg.Payload))
		})

		event := parseCardEvent(msg.Payload, boardAddr)
		log.Debugf("接收到控制器事件, DoorId: %d, Card: %s, EventType: %s", event.DoorId, event.CardNO, event.Type)

		data, err := json.Marshal(event)
		if nil != err {
			log.Error("JSON序列化错误", err)
			return
		}

		if err := trigger.PublishEvent(
			makeGroupId(byte(event.BoardId)),
			makeMajorId(int(event.DoorId)),
			directName(event.Direct),
			data,
			trigger.GenerateEventId()); nil != err {
			log.Error("触发事件出错: ", err)
			return
		}
	}

	// 读数据循环
	message := new(IrMessage)
	for {
		select {
		case <-shutdown:
			log.Debug("接收到系统终止信号")
			return nil

		default:
			err := cli.ReadWith(func(in io.Reader) error {
				if ok, err := ReadMessage(in, message); ok {
					process(message)
					return nil
				} else if err == ErrUnknownMessage {
					log.Debugf("接收到非艾润数据格式数据: DATA= %v", in)
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

func parseCardEvent(data []byte, boardAddr byte) extra.CardEvent {
	doorId := byte(0)
	switch data[9] & 0xF0 {
	case 0x10:
		doorId = 1
	case 0x20:
		doorId = 2
	case 0x30:
		doorId = 3
	case 0x40:
		doorId = 4
	}
	return extra.CardEvent{
		SerialNum: uint32(boardAddr),
		BoardId:   uint32(boardAddr),
		DoorId:    doorId,
		Direct:    extra.DirectIn,
		CardNO:    wg26.ParseFromWg26([3]byte{data[0], data[1], data[2]}).CardSN,
		Type:      extra.TypeCard,
		State:     "OPEN",
		Index:     0,
	}
}

// 创建TriggerNode消息函数
func FuncTriggerProperties(boardAddr byte, doorCount int) func() edgex.MainNodeProperties {
	deviceOf := func(doorId int, directName string) *edgex.VirtualNodeProperties {
		return &edgex.VirtualNodeProperties{
			GroupId:     makeGroupId(boardAddr),
			MajorId:     makeMajorId(doorId),
			MinorId:     directName,
			Description: fmt.Sprintf("控制器#%d-%d号门-%s-读卡器", boardAddr, doorId, directName),
			Virtual:     true,
		}
	}
	return func() edgex.MainNodeProperties {
		nodes := make([]*edgex.VirtualNodeProperties, doorCount*2)
		for d := 0; d < doorCount; d++ {
			nodes[d*2] = deviceOf(d+1, directName(extra.DirectIn))
			nodes[d*2+1] = deviceOf(d+1, directName(extra.DirectOut))
		}
		return edgex.MainNodeProperties{
			NodeType:     edgex.NodeTypeTrigger,
			Vendor:       VendorName,
			ConnDriver:   DriverName,
			VirtualNodes: nodes,
		}
	}
}
