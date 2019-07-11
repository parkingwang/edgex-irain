package irain

import (
	"bytes"
	"testing"
)

//
// Author: 陈哈哈 bitschen@163.com
//

func TestReadMessage(t *testing.T) {
	r := bytes.NewReader([]byte{0xE2, 0x56, 0x43, 0x3b, 0xff, 0xff, 0x01, 0x65, 0x62, 0x01, 0x12, 0xE3})
	msg := new(Message)
	ok, err := ReadMessage(r, msg)
	if !ok {
		t.Error("Not ok")
	}
	if nil != err {
		t.Error("Error: ", err)
	}
	if !bytes.Equal([]byte{0x56, 0x43, 0x3b, 0xff, 0xff, 0x01, 0x65, 0x62, 0x01, 0x12}, msg.Payload) {
		t.Error("Body not match")
	}
}

func TestReadMessageSuccess(t *testing.T) {
	r := bytes.NewReader([]byte{0xE2, 'Y', 0xE3})
	msg := new(Message)
	ok, err := ReadMessage(r, msg)
	if !ok {
		t.Error("Not ok")
	}
	if nil != err {
		t.Error("Error: ", err)
	}
	if !msg.IsSuccess() {
		t.Error("Body not match")
	}
}

func TestReadMessageError(t *testing.T) {
	r := bytes.NewReader([]byte{0xE2, 0xE3})
	msg := new(Message)
	ok, err := ReadMessage(r, msg)
	if ok {
		t.Error("Should not ok")
	}
	if nil == err {
		t.Error("Should return error")
	}
}
