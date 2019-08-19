package irain

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/yoojia/go-bytes"
	"io"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

var (
	ErrUnknownMessage = errors.New("unknown irain message bytes")
)

const (
	// 控制指令起始字节
	CommandStart byte = 0x00D2
	CommandEnd   byte = 0x00D3
	// 响应数据起始字节
	MessageStart byte = 0x00E2
	MessageEnd   byte = 0x00E3
)

// 控制指令ID列表
const (
	CmdIdRemoteOpen byte = 0x005A // 手动打开开关
	CmdIdEventScan  byte = 0x005B // 监控扫描
	CmdIdCardAdd    byte = 0x0052 // 添加卡
	CmdIdCardDelete byte = 0x0057 // 删除卡
	CmdIdCardClear  byte = 0x0050 // 清空卡
)

// 刷卡数据中进出方向定义
const (
	DirectIn  = 1
	DirectOut = 2
)

// 控制指令
type Command struct {
	start   byte   // 起始位
	Addr    byte   // 控制器地址
	Length  byte   // 数据长度
	CmdId   byte   // 指令ID
	Payload []byte // 指令载荷
	sum     byte   // 校验和
	end     byte   // 结束位
}

func (dk *Command) Bytes() []byte {
	// 使用小字节序
	br := bytes.NewWriter(binary.LittleEndian)
	br.NextByte(dk.start)
	br.NextByte(dk.Addr)
	br.NextByte(dk.Length)
	br.NextByte(dk.CmdId)
	br.NextBytes(dk.Payload[:])
	br.NextByte(dk.sum)
	// 控制器问题，需要删除最后一个D3字节
	// br.NextByte(dk.end)
	return br.Bytes()
}

// NewIrCommand 创建控制指令
// devAddr 控制器地址
// cmdId 指令ID
// payload 指令载荷
func NewIrCommand(devAddr, cmdId byte, payload []byte) *Command {
	size := byte(len(payload))
	// 计算XOR校验和
	sum := devAddr
	sum ^= size
	sum ^= cmdId
	for _, b := range payload {
		sum ^= b
	}
	return &Command{
		start:   CommandStart,
		Addr:    devAddr,
		Length:  size,
		CmdId:   cmdId,
		Payload: payload,
		sum:     sum,
		end:     CommandEnd,
	}
}

////

// 响应数据结构
type Message struct {
	start   byte // 起始位
	Payload []byte
	end     byte
}

func (r *Message) IsSuccess() bool {
	return 'Y' == r.Payload[0]
}

// ReadMessage 从Reader读取响应数据字节，通过填充 Message 结构返回结果。
// 如果读取Reply结构成功，返回True标记位；失败，则返回False，并返回 ErrUnknownMessage 错误。
func ReadMessage(in io.Reader, out *Message) (ok bool, err error) {
	reader := bufio.NewReader(in)
	// Wait start byte
	for {
		b, err := reader.ReadByte()
		if nil != err {
			return false, err
		}
		if b == MessageStart {
			break
		}
	}
	// read to the end of reply
	data, err := reader.ReadBytes(MessageEnd)
	if nil != err {
		return false, err
	}
	size := len(data)
	// 包括 MessageEnd 至少2个字节
	if 2 > size {
		return false, ErrUnknownMessage
	}
	out.start = MessageStart
	out.Payload = data[:size-1]
	out.end = MessageEnd
	return true, nil
}

////

func DirectName(dir byte) string {
	if 1 == dir {
		return "IN"
	} else {
		return "OUT"
	}
}

func makeGroupId(ctrlId byte) string {
	return fmt.Sprintf("ADDR#%d", ctrlId)
}

func makeDoorId(doorId int) string {
	return fmt.Sprintf("DOOR#%d", doorId)
}
