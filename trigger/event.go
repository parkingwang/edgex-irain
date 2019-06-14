package main

import (
	"github.com/bitschen/go-jsonx"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/parkingwang/go-wg26"
	"github.com/pkg/errors"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

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
func parseEvent(devAddr byte, data []byte) (*Event, error) {
	if len(data) != 12 {
		return nil, errors.New("INVALID_IRAIN_EVENT")
	}
	// [0] 		e2
	// [1-3]    a9 bc bf :维根26格式的卡号
	// [4-9] 	ff ff 01 65 62 01 // 控制器时间
	// [10]		12 门号
	// [11]		e3
	door := byte(0)
	switch data[10] & 0xF0 {
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
		Card:         wg26.ParseFromWg26([3]byte{data[1], data[2], data[3]}),
		ControllerId: devAddr,
		State:        1,
		DoorId:       door,
		Direct:       irain.DirectIn,
	}, nil
}
