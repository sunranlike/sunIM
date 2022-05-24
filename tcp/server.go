package tcp

import (
	"context"
	"errors"
	"fmt"
	sun "github.com/sunrnalike/sun"
	"net"
	"sync"
	"time"

	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"

	"github.com/segmentio/ksuid"
)

// ServerOptions ServerOptions
type ServerOptions struct {
	loginwait time.Duration //登陆超时
	readwait  time.Duration //读超时
	writewait time.Duration //读超时
}

// Server is a websocket implement of the Server
type Server struct {
	listen string
	naming.ServiceRegistration
	sun.ChannelMap
	sun.Acceptor
	sun.MessageListener
	sun.StateListener
	once    sync.Once
	options ServerOptions
	quit    *sun.Event
}

// NewServer NewServer
func NewServer(listen string, service naming.ServiceRegistration) sun.Server {
	return &Server{
		listen:              listen,
		ServiceRegistration: service,
		ChannelMap:          sun.NewChannels(100),
		quit:                sun.NewEvent(),
		options: ServerOptions{
			loginwait: sun.DefaultLoginWait,
			readwait:  sun.DefaultReadWait,
			writewait: time.Second * 10,
		},
	}
}

// Start server
func (s *Server) Start() error {
	log := logger.WithFields(logger.Fields{
		"module": "tcp.server",
		"listen": s.listen,
		"id":     s.ServiceID(),
	})

	if s.StateListener == nil {
		return fmt.Errorf("StateListener is nil")
	}
	if s.Acceptor == nil {
		s.Acceptor = new(defaultAcceptor)
	}

	lst, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	log.Info("started")
	for {
		rawconn, err := lst.Accept()
		if err != nil {
			rawconn.Close()
			log.Warn(err)
			continue
		}
		go func(rawconn net.Conn) {
			conn := NewConn(rawconn)

			id, err := s.Accept(conn, s.options.loginwait)
			if err != nil {
				_ = conn.WriteFrame(sun.OpClose, []byte(err.Error()))
				conn.Close()
				return
			}
			if _, ok := s.Get(id); ok {
				log.Warnf("channel %s existed", id)
				_ = conn.WriteFrame(sun.OpClose, []byte("channelId is repeated"))
				conn.Close()
				return
			}

			channel := sun.NewChannel(id, conn)
			channel.SetReadWait(s.options.readwait)
			channel.SetWriteWait(s.options.writewait)

			s.Add(channel)

			log.Info("accept ", channel)
			err = channel.Readloop(s.MessageListener)
			if err != nil {
				log.Info(err)
			}
			s.Remove(channel.ID())
			_ = s.Disconnect(channel.ID())
			channel.Close()
		}(rawconn)

		select {
		case <-s.quit.Done():
			return fmt.Errorf("listen exited")
		default:
		}
	}

}

// Shutdown Shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	log := logger.WithFields(logger.Fields{
		"module": "tcp.server",
		"id":     s.ServiceID(),
	})
	s.once.Do(func() {
		defer func() {
			log.Infoln("shutdown")
		}()
		// close channels
		chanels := s.ChannelMap.All()
		for _, ch := range chanels {
			ch.Close()

			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

	})
	return nil
}

// string channelID
// []byte data
func (s *Server) Push(id string, data []byte) error {
	ch, ok := s.ChannelMap.Get(id)
	if !ok {
		return errors.New("channel no found")
	}
	return ch.Push(data)
}

// SetAcceptor SetAcceptor
func (s *Server) SetAcceptor(acceptor sun.Acceptor) {
	s.Acceptor = acceptor
}

// SetMessageListener SetMessageListener
func (s *Server) SetMessageListener(listener sun.MessageListener) {
	s.MessageListener = listener
}

// SetStateListener SetStateListener
func (s *Server) SetStateListener(listener sun.StateListener) {
	s.StateListener = listener
}

// SetReadWait set read wait duration
func (s *Server) SetReadWait(readwait time.Duration) {
	s.options.readwait = readwait
}

// SetChannels SetChannels
func (s *Server) SetChannelMap(channels sun.ChannelMap) {
	s.ChannelMap = channels
}

type defaultAcceptor struct {
}

// Accept defaultAcceptor
func (a *defaultAcceptor) Accept(conn sun.Conn, timeout time.Duration) (string, error) {
	return ksuid.New().String(), nil
}
