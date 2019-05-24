# iRain - 艾润品牌设备驱动

## Trigger - 事件触发器

**程序配置**

```toml
# 顶级必要的配置参数
Name = "iRainTrigger"
Topic = "irain/events"

[BoardOptions]
  controllerId = 1
  monitorPeriod = "1s"

# Socket配置参数
[SocketClientOptions]
  targetAddress = "tcp://board.ir0.edgex.io:50000"
  bufferSize = 1024
  readTimeout = "1s"
  writeTimeout = "1s"
  autoReconnect = true
  reconnectInterval = "3s"
  keepAlive = true
  keepAliveInterval = "3s"
```


配置说明：

- `Name` 设备名称，在项目内部每个设备名称必须保持唯一性；
- `Topic` 每个Trigger都必须指定一个Topic；不得以`/`开头；
- `SocketClientOptions.targetAddress` 连接门禁控制器服务端地址；

#### 程序说明

Trigger启动后，等待控制器连接到程序的TCP服务端，并接收其刷卡广播数据。
接收到刷卡数据后，将数据生成以下JSON格式数据包，以指定的Topic发送到MQTT服务器。

消息Name格式：

> {controllerId}/{doorId]/{direct}

消息数据格式：

```json
{
  "sn": 123,
  "card": 123,
  "cardHex": "a1b2c3d4",
  "index": 123,
  "type": 1,
  "doorId": 1,
  "direct": 1,
  "state": 1
}
```

- `sn` 设备序列号；
- `card` 卡号。uint32类型；
- `cardHex` 卡号。hex类型；
- `doorId` 刷卡门号；
- `direct` 刷卡门号；
- `state` 刷卡状态；



