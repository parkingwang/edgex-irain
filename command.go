package irain

import (
	"encoding/binary"
	"github.com/nextabc-lab/edgex-go"
)

//
// Author: 陈哈哈 chenyongjia@parkingwang.com, yoojiachen@gmail.com
//
const (
	MagicStart byte = 0x00F3
	MagicEnd   byte = 0x00D3
)

const (
	FunRemoteOpen  byte = 0x005A // 手动打开开关
	FunMonitorScan byte = '['    // 监控扫描
)

type Command struct {
	magicStart byte // 起始位
	addr       byte // 控制器地址
	length     byte // 数据长度
	funId      byte // 指令
	data       []byte
	sum        byte
	magicEnd   byte
}

func (dk *Command) Bytes() []byte {
	// 使用小字节序
	br := edgex.NewByteWriter(binary.LittleEndian)
	br.PutByte(dk.magicStart)
	br.PutByte(dk.addr)
	br.PutByte(dk.length)
	br.PutByte(dk.funId)
	br.PutBytes(dk.data[:])
	br.PutByte(dk.sum)
	br.PutByte(dk.magicEnd)
	return br.Bytes()
}

func NewCommand(addr, funId byte, data []byte) *Command {
	dataLen := byte(len(data))
	// 计算XOR校验和
	sum := addr
	sum ^= dataLen
	sum ^= funId
	for _, b := range data {
		sum ^= b
	}
	return &Command{
		magicStart: MagicStart,
		addr:       addr,
		length:     dataLen,
		funId:      funId,
		data:       data,
		sum:        sum,
		magicEnd:   MagicEnd,
	}
}
