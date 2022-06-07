# sunIM

sunIM CLoud

## 简介

    sun 是一个高性能分式式通信架构。 
    使用了雪花算法、rest、protobuf、wobwas、gorm
    技术选型：mysql、redis、consul、

## 代码演示

首先进入examples目录：

1. **启用服务端**

```cmd
go run main.go mock_srv -p ws
INFO[0000] started                                       id=srv1 listen=":8000" module=ws.server
```

2. **启用客户端**

```cmd
$ go run main.go mock_cli -p ws
WARN[0000] 1uWbA9ajf86A44J8t4k2AtsadQG receive message [hello from server ] 
WARN[0001] 1uWbA9ajf86A44J8t4k2AtsadQG receive message [hello from server ] 
WARN[0002] 1uWbA9ajf86A44J8t4k2AtsadQG receive message [hello from server ] 
WARN[0003] 1uWbA9ajf86A44J8t4k2AtsadQG receive message [hello from server ] 
...
```
