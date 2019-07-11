package main

import (
	"errors"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/parkingwang/go-wg26"
	"github.com/yoojia/go-jsonx"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

var (
	ErrUnknownCardEvent = errors.New("unknown card event")
)

// 刷卡数据
type Event struct {
	Card         *wg26.Wg26Id // 卡号
	ControllerId byte         // 控制器ID
	State        byte         // 开门状态
	DoorId       byte
	Direct       byte
}

func (e *Event) Bytes() []byte {
	return jsonx.NewFatJSON().
		Field("sn", e.ControllerId).
		Field("index", 0).
		Field("type", 0).
		Field("typeName", "CARD").
		Field("state", e.State).
		Field("card", e.Card.CardSN).
		Field("doorId", e.DoorId).
		Field("direct", irain.DirectName(e.Direct)).Bytes()
}

// 解析刷卡数据
func parseCardEvent(devAddr byte, payload []byte) (*Event, error) {
	if 10 != len(payload) {
		return nil, ErrUnknownCardEvent
	}
	// [0-2]    a9 bc bf :维根26格式的卡号
	// [3-8] 	ff ff 01 65 62 01 // 控制器时间
	// [9]		门号
	door := byte(0)
	switch payload[9] & 0xF0 {
	case 0x10:
		door = 1
	case 0x20:
		door = 2
	case 0x30:
		door = 3
	case 0x40:
		door = 4
	}
	return &Event{
		Card:         wg26.ParseFromWg26([3]byte{payload[0], payload[1], payload[2]}),
		ControllerId: devAddr,
		State:        1,
		DoorId:       door,
		Direct:       irain.DirectIn,
	}, nil
}
