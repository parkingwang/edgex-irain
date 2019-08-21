package main

import (
	"flag"
	"github.com/nextabc-lab/edgex-go"
	"github.com/nextabc-lab/edgex-irain"
	"github.com/yoojia/go-at"
	"github.com/yoojia/go-socket"
	"github.com/yoojia/go-value"
	"time"
)

//
// Author: 陈哈哈 yoojiachen@gmail.com
// 使用Socket客户端连接的Trigger。注意与Endpoint都是使用Client模式。

func main() {
	edgex.Run(irainApp)
}

func irainApp(ctx edgex.Context) error {
	fileName := flag.String("c", edgex.DefaultConfName, "config file name")
	config := ctx.LoadConfigByName(*fileName)
	// Init
	log := ctx.Log()
	ctx.InitialWithConfig(config)

	eventTopic := value.Of(config["Topic"]).String()

	boardOpts := value.Of(config["BoardOptions"]).MustMap()
	boardAddr := byte(value.Of(boardOpts["controllerId"]).MustInt64())
	doorCount := value.Of(boardOpts["doorCount"]).Int64OrDefault(1)

	// Socket客户商连接
	clientOpts := value.Of(config["SocketClientOptions"]).MustMap()
	remoteAddress := value.Of(clientOpts["remoteAddress"]).String()
	network := value.Of(clientOpts["network"]).StringOrDefault("tcp")
	cli := sock.New(sock.Options{
		Network:           network,
		Addr:              remoteAddress,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		KeepAlive:         true,
		KeepAliveInterval: time.Second,
		ReconnectDelay:    time.Second,
	})
	log.Debugf("客户端连接: [%s] %s", network, remoteAddress)
	if err := cli.Connect(); nil != err {
		log.Error("客户端连接失败", err)
	} else {
		log.Debug("客户端连接成功")
	}
	defer func() {
		if err := cli.Disconnect(); nil != err {
			log.Error("客户端关闭连接失败：", err)
		}
	}()

	// AT指令解析
	registry := at.NewRegistry()
	irain.AtCommands(registry, boardAddr)

	// Trigger服务，监听客户端数据
	trigger := ctx.NewTrigger(edgex.TriggerOptions{
		Topic:              eventTopic,
		NodePropertiesFunc: irain.FuncTriggerProperties(boardAddr, int(doorCount)),
	})

	// Endpoint服务
	endpoint := ctx.NewEndpoint(edgex.EndpointOptions{
		NodePropertiesFunc: irain.FuncEndpointProperties(boardAddr, int(doorCount)),
	})
	endpoint.Serve(irain.FuncRpcServe(ctx, registry, cli))

	// 启动服务
	trigger.Startup()
	defer trigger.Shutdown()

	endpoint.Startup()
	defer endpoint.Shutdown()

	// 监听接收消息循环
	return irain.ReceiveLoop(ctx, trigger, boardAddr, cli, ctx.TermChan())

}
