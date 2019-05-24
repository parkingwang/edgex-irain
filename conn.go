package irain

import (
	"github.com/yoojia/go-value"
	"strings"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
//

func NewClientWithOptions(targetAddress string, sockOpts map[string]interface{}) *SockClient {
	network := "tcp"
	address := targetAddress
	if strings.Contains(address, "://") {
		network = strings.Split(address, "://")[0]
		address = strings.Split(address, "://")[1]
	}
	return NewSockClient(SockOptions{
		Network:           network,
		Addr:              address,
		BufferSize:        uint(value.Of(sockOpts["bufferSize"]).Int64OrDefault(1024)),
		ReadTimeout:       value.Of(sockOpts["readTimeout"]).DurationOfDefault(time.Second),
		WriteTimeout:      value.Of(sockOpts["writeTimeout"]).DurationOfDefault(time.Second),
		AutoReconnect:     value.Of(sockOpts["autoReconnect"]).BoolOrDefault(true),
		ReconnectInterval: value.Of(sockOpts["reconnectInterval"]).DurationOfDefault(time.Second * 3),
		KeepAlive:         value.Of(sockOpts["keepAlive"]).BoolOrDefault(true),
		KeepAliveInterval: value.Of(sockOpts["keepAliveInterval"]).DurationOfDefault(time.Second * 3),
	})
}