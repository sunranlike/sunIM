package tcp

import (
	"context"
	"errors"
	"fmt"
	"github.com/sunrnalike/sun"
	"net"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/sunrnalike/sun/logger"
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
	sun.ServiceRegistration
	sun.ChannelMap
	sun.Acceptor
	sun.MessageListener
	sun.StateListener
	once    sync.Once
	options ServerOptions
	quit    *sun.Event
}

// NewServer NewServer
func NewServer(listen string, service sun.ServiceRegistration) sun.Server {
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
	log := logger.WithFields(logger.Fields{ //日志初始化
		"module": "tcp.server",
		"listen": s.listen,
		"id":     s.ServiceID(),
	})
	//判断先行
	if s.StateListener == nil {
		return fmt.Errorf("StateListener is nil")
	}
	if s.Acceptor == nil {
		s.Acceptor = new(defaultAcceptor)
	}
	//启用监听TCP服务,所有对这个url的tcp服务都会传入lst,
	//比如说有人curl了s.listen,那么lst.Accept就会返回这个链接，这里会阻塞吗？
	lst, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	log.Info("started")
	for { //开启无尽循环,执行任务
		rawconn, err := lst.Accept() //首先接受连接(当有人curl这个链接)
		if err != nil {              //判断先行
			rawconn.Close()
			log.Warn(err)
			continue //这里直接下一次循环,不开启routine,因为没有连接啊,所以要continue,不走下面
		}
		//以下理解错误:***
		//这里直接开启routine,并且没有判断,如果没有链接吗,既lst.accept没有返回一个连接,这里立马就会返回
		//也就是说在没有连接的时候,就是一个重复的新建routine,然后杀死return routine杀死它,重复的建立和消灭
		//***以上理解错误

		//这里在有链接的时候就开启routine处理
		go func(rawconn net.Conn) {
			conn := NewConn(rawconn) //从rawconn中获取conn,这也是个构造函数
			//这样conn就实现了sun.Conn接口,可以调用Conn方法

			//调用上层去实现验证鉴权等逻辑
			id, err := s.Accept(conn, s.options.loginwait)
			if err != nil {
				_ = conn.WriteFrame(sun.OpClose, []byte(err.Error())) //返回一个close,不然客户端傻等
				conn.Close()
				return
			}
			if _, ok := s.Get(id); ok { //判断是否在channel map中存在这个id的连接
				log.Warnf("channel %s existed", id)
				_ = conn.WriteFrame(sun.OpClose, []byte("channelId is repeated")) //也是让客户端知道重复
				conn.Close()
				return
			}
			//不重复,不error,下面开始新建channel并且set 两个wait时间
			channel := sun.NewChannel(id, conn) //这个NewChannel也是个构造函数
			channel.SetReadWait(s.options.readwait)
			channel.SetWriteWait(s.options.writewait)
			//新建之后自然要add到channelmap之中
			s.Add(channel)

			log.Info("accept ", channel)
			//开启readloop,带loop的都是阻塞方法,会不停的等待读取数据
			//直到客户端close
			err = channel.Readloop(s.MessageListener)

			//以下是close之后的操作
			if err != nil {
				log.Info(err)
			}
			//close后先remove id,然后关闭连接,然后关闭channel
			s.Remove(channel.ID())
			_ = s.Disconnect(channel.ID())
			channel.Close()
		}(rawconn)

		select { //上面开了一个routine去读取数据readloop,这里也是阻塞,只到s.quit.Done(),也就是读取结束
		case <-s.quit.Done():
			return fmt.Errorf("listen exited")
		default:
		}
	}

}

// Shutdown Shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	log := logger.WithFields(logger.Fields{ //日志
		"module": "tcp.server",
		"id":     s.ServiceID(),
	})
	s.once.Do(func() { //使用once的Do: 只shutdown一次
		defer func() {
			log.Infoln("shutdown")
		}()
		// close channels
		chanels := s.ChannelMap.All() //获取channem slice
		for _, ch := range chanels {  //遍历并且关闭
			ch.Close() //每个ch都关掉

			select {
			case <-ctx.Done(): //如果说调用的时候传入的ctx已经Done了,那么久直接return了,不用再close了
				return
			default: //如果没有上下文Done,那么久继续下次循环关闭下个chan,只到所有的都关闭
				continue
			}
		}

	})
	return nil
}

// string channelID
// []byte data
//Push 复用低层push
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
