package irain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nextabc-lab/edgex-go"
	"github.com/yoojia/go-at"
	sock "github.com/yoojia/go-socket"
	"go.uber.org/zap"
	"io"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

var (
	RepOK   = errors.New("EX=OK")
	RepFail = errors.New("EX=ERR:FAILED")
	RepNop  = errors.New("EX=ERR:NO_VALID_REPLY")
)

// 创建RPC服务函数
func FuncRpcServe(ctx edgex.Context, atRegistry *at.AtRegister, cli *sock.Client) func(msg edgex.Message) (out []byte) {
	log := ctx.Log()
	return func(msg edgex.Message) (out []byte) {
		atCmd := string(msg.Body())
		log.Debug("接收到控制指令: " + atCmd)
		cmd, err := atRegistry.Apply(atCmd)
		if nil != err {
			log.Error("控制指令格式错误: " + err.Error())
			return []byte("EX=ERR:BAD_CMD:" + err.Error())
		}
		ctx.LogIfVerbose(func(log *zap.SugaredLogger) {
			log.Debug("艾润控制指令码: " + hex.EncodeToString(cmd))
		})
		// Write
		if _, err := cli.Write(cmd); nil != err {
			log.Error("发送/写入控制指令出错", err)
			return []byte("EX=ERR:WRITE:" + err.Error())
		}
		reply := tryReadReply(ctx, cli)
		return []byte(reply)
	}
}

// 创建EndpointNode函数
func FuncEndpointProperties(controllerId byte, doorCount int) func() edgex.MainNodeProperties {
	deviceOf := func(doorId int) *edgex.VirtualNodeProperties {
		return &edgex.VirtualNodeProperties{
			VirtualId:   fmt.Sprintf("SWITCH-%d-%d", controllerId, doorId),
			MajorId:     fmt.Sprintf("%d", controllerId),
			MinorId:     fmt.Sprintf("%d", doorId),
			Description: fmt.Sprintf("%d号门-电磁开关", doorId),
			Virtual:     true,
			StateCommands: map[string]string{
				"TRIGGER": fmt.Sprintf("AT+OPEN=%d", doorId),
			},
		}
	}
	return func() edgex.MainNodeProperties {
		nodes := make([]*edgex.VirtualNodeProperties, doorCount)
		for d := 0; d < doorCount; d++ {
			nodes[d] = deviceOf(d + 1)
		}
		return edgex.MainNodeProperties{
			NodeType:     edgex.NodeTypeEndpoint,
			Vendor:       VendorName,
			ConnDriver:   DriverName,
			VirtualNodes: nodes,
		}
	}
}

// 读取设备响应数据
// 只读取应答指令，忽略其它指令。最多读取5次，间隔100毫秒
func tryReadReply(ctx edgex.Context, cli *sock.Client) string {
	log := ctx.Log()
	msg := new(Message)
	for i := 0; i < 5; i++ {
		err := cli.ReadWith(func(in io.Reader) error {
			if ok, e := ReadMessage(in, msg); !ok {
				return e
			} else if msg.IsSuccess() {
				return RepOK
			} else {
				return RepFail
			}
		})
		if err != RepOK && err != RepFail {
			log.Errorf("读取设备响应数据出错[%d]: %s", i, err)
			<-time.After(time.Millisecond * 100)
			continue
		} else {
			if nil == err {
				return RepNop.Error()
			} else {
				return err.Error()
			}
		}
	}
	return RepNop.Error()
}
