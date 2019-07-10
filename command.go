package irain

import (
	"encoding/binary"
	"github.com/yoojia/go-bytes"
)

//
// Author: 陈哈哈 chenyongjia@parkingwang.com, yoojiachen@gmail.com
//
const (
	MagicStart byte = 0x00D2
	MagicEnd   byte = 0x00D3
	DataStart  byte = 0x00E2
	DataEnd    byte = 0x00E3
)

const (
	CmdRemoteOpen byte = 0x005A // 手动打开开关
	CmdEventScan  byte = 0x005B // 监控扫描
	CmdCardAdd    byte = 0x0052 // 添加卡
	CmdCardDelete byte = 0x0057 // 删除卡
	CmdCardClear  byte = 0x0050 // 清空卡
)

const (
	DirectIn  = 1
	DirectOut = 2
)

type Command struct {
	magicStart byte // 起始位
	addr       byte // 控制器地址
	length     byte // 数据长度
	cmdId      byte // 指令
	data       []byte
	sum        byte
	magicEnd   byte
}

func (dk *Command) Bytes() []byte {
	// 使用小字节序
	br := bytes.NewWriter(binary.LittleEndian)
	br.NextByte(dk.magicStart)
	br.NextByte(dk.addr)
	br.NextByte(dk.length)
	br.NextByte(dk.cmdId)
	br.NextBytes(dk.data[:])
	br.NextByte(dk.sum)
	br.NextByte(dk.magicEnd)
	return br.Bytes()
}

func NewCommand(devAddr, cmdId byte, data []byte) *Command {
	dataLen := byte(len(data))
	// 计算XOR校验和
	sum := devAddr
	sum ^= dataLen
	sum ^= cmdId
	for _, b := range data {
		sum ^= b
	}
	return &Command{
		magicStart: MagicStart,
		addr:       devAddr,
		length:     dataLen,
		cmdId:      cmdId,
		data:       data,
		sum:        sum,
		magicEnd:   MagicEnd,
	}
}

// 检查数据字节是否为协议数据
func CheckProtoValid(data []byte) bool {
	size := len(data)
	if size > 2 && DataStart == data[0] && DataEnd == data[size-1] {
		return true
	} else {
		return false
	}
}

func DirectName(dir byte) string {
	if 1 == dir {
		return "IN"
	} else {
		return "OUT"
	}
}
