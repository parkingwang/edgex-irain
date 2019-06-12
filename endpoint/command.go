package main

import (
	"encoding/binary"
	"errors"
	"github.com/nextabc-lab/edgex-dongkong"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-bytes"
	"strconv"
)

//
// Author: 陈哈哈 bitschen@163.com
//

func atCommands(registry *at.AtRegister, devAddr byte) {
	// AT+OPEN=SWITCH_ID
	registry.AddX("OPEN", 1, func(args ...string) ([]byte, error) {
		switchId, err := parseInt(args[0])
		if nil != err {
			return nil, errors.New("INVALID_SWITCH_ID:" + args[0])
		}
		return newCommandRemoteOpen(devAddr, byte(switchId)).Bytes(), nil
	})
	// AT+ADD=CARD(uint32)
	addHandler := func(args ...string) ([]byte, error) {
		card, err := getCardNumber(args[0])
		if nil != err {
			return nil, err
		}
		/*
			5—10:卡号
			11—18:截止日期(年、月、日、时)
			19:开门有效否?(bit0=’0’:开门无效， bit0=’1’:开门有效)
			20:密码，反潜否?(bit0=’0’:密码无效， bit0=’1’:密码有效 bit1=’0’:反潜无效， bit1=’1’:反潜有效)
			21—24:人员编号
			25、26:班组
			27、28:星期有效否?(bit0=’0’:星期日无效; bit0=’1’:星期日有效; bit1=’0’:星 期 1 无效; bit1=’1’:星期 1 有效;bit2=’0’:星期 2 无效， bit0=’1’:星期 2 有效:...... )
			29、30:大门有效否?(bit0=’0’:大门 1 无效， bit0=’1’:大门 1 有效:
				bit1=’0’:大门 2 无效; bit2=’0’:大门 3 无效; bit3=’0’:大门 4 无效;
				bit1=’1’:大门 2 有效; bit2=’1’:大门 3 有效 bit3=’1’:大门 4 有效)
		*/
		w := bytes.NewWriter(binary.LittleEndian)
		// 5—10:卡号: TODO Hex值
		w.NextUint32(card)
		// 11—18:截止日期: 全是F,不限制
		w.NextBytes([]byte{'F', 'F', 'F', 'F'})
		// 19:开门 密码
		w.NextBytes([]byte{'1', '0'})
		// 21—24:人员编号
		w.NextUint32(0)
		// 25、26: 班组
		w.NextBytes([]byte{'0', '1'})
		// 27、28:星期
		w.NextBytes([]byte{'7', 'F'})
		// 29、30:大门
		w.NextBytes([]byte{'0', '1'})
		return irain.NewCommand(devAddr, irain.CmdCardAdd, w.Bytes()).Bytes(), nil
	}
	registry.AddX("ADD", 1, addHandler)
	registry.Add("ADD0", addHandler)

	// AT+DELETE=CARD(uint32)
	registry.AddX("DELETE", 1, func(args ...string) ([]byte, error) {
		card, err := getCardNumber(args[0])
		if nil != err {
			return nil, err
		}
		return NewCommandCardDelete(devAddr, card).Bytes(), nil
	})

	// AT+CLEAR
	registry.AddX("CLEAR", 0, func(args ...string) ([]byte, error) {
		return NewCommandCardClear(devAddr).Bytes(), nil
	})
}

// 创建远程开门指令
func newCommandRemoteOpen(devAddr, doorId byte) *irain.Command {
	return irain.NewCommand(devAddr, irain.CmdRemoteOpen, []byte{doorId})
}

// 创建清除卡号指令
func NewCommandCardClear(devAddr byte) *irain.Command {
	return irain.NewCommand(devAddr, irain.CmdCardClear, []byte{
		0x0078, 0x0079, 0x007A,
	})
}

// 创建删除卡号指令
func NewCommandCardDelete(devAddr byte, card uint32) *irain.Command {
	return irain.NewCommand(devAddr, irain.CmdCardDelete, cardToByte6(card))
}

func cardToByte6(card uint32) []byte {
	// 协议中卡号为 6字节
	data := bytes.NewWriter(binary.LittleEndian)
	data.NextUint32(card) // 4字节卡号
	data.NextUint16(0)    // 补充2字节
	return data.Bytes()
}

func getCardNumber(val string) (uint32, error) {
	intCardNum, err := parseInt(val)
	if nil != err {
		return 0, errors.New("INVALID_CARD: " + val)
	} else {
		return uint32(intCardNum), nil
	}
}

func parseInt(val string) (int64, error) {
	return strconv.ParseInt(val, 10, 64)
}
