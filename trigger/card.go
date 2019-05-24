package main

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

// 刷卡数据
type Event struct {
	Card    [3]byte // 卡号
	Index   uint16  // 序列号
	BoardId byte    // 控制器ID
	Group   byte    // 班组
	State   byte    // 开门状态
	Doors   byte
	Direct  byte
}

func (e Event) Bytes() []byte {
	// TODO
	return []byte{}
}

// 解析刷卡数据
func parseEvent(data []byte) (Event, error) {
	// TODO
	return Event{}, nil
}
