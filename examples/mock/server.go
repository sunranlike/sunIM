package mock

import (
	"errors"
	sun "github.com/sunrnalike/sun"
	"time"

	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"
	"github.com/sunrnalike/sun/tcp"
	"github.com/sunrnalike/sun/websocket"
)

type ServerDemo struct{}

func (s *ServerDemo) Start(id, protocol, addr string) {
	var srv sun.Server
	service := &naming.DefaultService{
		Id:       id,
		Protocol: protocol,
	}
	if protocol == "ws" {
		srv = websocket.NewServer(addr, service)
	} else if protocol == "tcp" {
		srv = tcp.NewServer(addr, service)
	}

	handler := &ServerHandler{}

	srv.SetReadWait(time.Minute)
	srv.SetAcceptor(handler)
	srv.SetMessageListener(handler)
	srv.SetStateListener(handler)

	err := srv.Start()
	if err != nil {
		panic(err)
	}
}

// ServerHandler ServerHandler
type ServerHandler struct {
}

// Accept this connection
func (h *ServerHandler) Accept(conn sun.Conn, timeout time.Duration) (string, error) {
	// 1. 读取：客户端发送的鉴权数据包
	frame, err := conn.ReadFrame()
	if err != nil {
		return "", err
	}
	logger.Info("recv", frame.GetOpCode())
	// 2. 解析：数据包内容就是userId
	userID := string(frame.GetPayload())
	// 3. 鉴权：这里只是为了示例做一个fake验证，非空
	if userID == "" {
		return "", errors.New("user id is invalid")
	}
	return userID, nil
}

// Receive default listener
func (h *ServerHandler) Receive(ag sun.Agent, payload []byte) {
	ack := string(payload) + " from server "
	_ = ag.Push([]byte(ack))
}

// Disconnect default listener
func (h *ServerHandler) Disconnect(id string) error {
	logger.Warnf("disconnect %s", id)
	return nil
}
