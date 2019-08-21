package irain

import (
	"encoding/binary"
	"errors"
	"github.com/parkingwang/go-wg26"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-bytes"
)

//
// Author: 陈哈哈 bitschen@163.com
//

func AtCommands(registry *at.Registry, devAddr byte) {
	// AT+OPEN=SWITCH_ID
	registry.AddX("OPEN", 1, func(args at.Args) ([]byte, error) {
		switchId, err := args.ArgInt64(0)
		if nil != err {
			return nil, errors.New("INVALID_SWITCH_ID:" + args[0])
		}
		return newCommandRemoteOpen(devAddr, byte(switchId)).Bytes(), nil
	})
	// AT+ADD=CARD(SN)
	/*
		5-10: 卡号
		11-18: 截止日期(年、月、日、时)
		19: 开门有效
		20: 密码，反潜
		21-24:人员编号
		25、26:班组
		27、28:星期有效否
		29、30: 大门有效否
	*/
	addHandler := func(args at.Args) ([]byte, error) {
		card := args[0]
		if !wg26.IsCardSN(card) {
			return nil, errors.New("INVALID_CARD_SN[10digits]")
		}
		wg26id := wg26.ParseFromCardNumber(card)
		w := bytes.NewWriter(binary.BigEndian)
		// 5-10:卡号:6位
		cardBytes := []byte(wg26id.CardSN)
		w.NextBytes(cardBytes)
		// 11-18:截止日期: 全是F,不限制
		w.NextBytes([]byte{'F', 'F', 'F', 'F'})
		// 19:开门 密码
		w.NextBytes([]byte{'1', '0'})
		// 21-24:人员编号
		w.NextUint32(0)
		// 25、26: 班组
		w.NextBytes([]byte{'0', '1'})
		// 27、28:星期
		w.NextBytes([]byte{'7', 'F'})
		// 29、30:大门
		w.NextBytes([]byte{'0', '1'})
		return NewIrCommand(devAddr, CmdIdCardAdd, w.Bytes()).Bytes(), nil
	}
	registry.AddX("ADD", 1, addHandler)
	registry.Add("ADD0", addHandler)

	// AT+DELETE=CARD(SN)
	registry.AddX("DELETE", 1, func(args at.Args) ([]byte, error) {
		card := args[0]
		if !wg26.IsCardSN(card) {
			return nil, errors.New("INVALID_CARD_SN[10digits]")
		}
		wg26id := wg26.ParseFromCardNumber(card)
		cardBytes := []byte(wg26id.CardSN)
		return NewCommandCardDelete(devAddr, cardBytes).Bytes(), nil
	})

	// AT+CLEAR
	registry.AddX("CLEAR", 0, func(args at.Args) ([]byte, error) {
		return NewCommandCardClear(devAddr).Bytes(), nil
	})
}

// 创建远程开门指令
func newCommandRemoteOpen(devAddr, doorId byte) *Command {
	return NewIrCommand(devAddr, CmdIdRemoteOpen, []byte{doorId})
}

// 创建清除卡号指令
func NewCommandCardClear(devAddr byte) *Command {
	return NewIrCommand(devAddr, CmdIdCardClear, []byte{
		0x0078, 0x0079, 0x007A,
	})
}

// 创建删除卡号指令
func NewCommandCardDelete(devAddr byte, card []byte) *Command {
	return NewIrCommand(devAddr, CmdIdCardDelete, card)
}
