package sun

import (
	"context"
	"net"
	"time"
)

const (
	DefaultReadWait  = time.Minute * 3
	DefaultWriteWait = time.Second * 10
	DefaultLoginWait = time.Second * 10
	DefaultHeartbeat = time.Second * 55
)

//有关service我们有许多接口:service,serviceRegistration,server
//他们之前彼此有不同的作用
//1 service是一个小接口,它主要是服务端的抽象接口
//2 serviceRegistration是一个服务注册的接口,必须实现这个接口才可以传入consul进行服务注册
//同时他被强制植入了service,强制实现
//3 然后就是server接口,这个接口

// 定义了基础服务的抽象接口
//
type Service interface {
	ServiceID() string
	ServiceName() string
	GetMeta() map[string]string
}

// 定义服务注册的抽象接口,
//这个接口是干嘛的？
//我们知道服务注册是将被注册的服务注册入某个服务容器中
//所有想要 被注册进去的容器的都 需要实现这个接口,才可以被注册进入服务中心
//

//类比的话有点像serviceproivdr,那么这个玩意会在哪里使用?
//答案:在naming.consul中,会被使用去注册.每当你注册一个服务的时候,调用c.naming.Register的时候
//就会把一个实现了这个对象的struct当做参数传入容器,注册中心consul会调用这个参数的sr接口的方法
//比如说err := c.Naming.Register(c.Srv),在这个方法中传入的是一个实现了SR接口的大佬,然后
//就会调用conmsul,将这个c.Srv的一些方法传入consul,就实现了注册入服务中心的功能.

type ServiceRegistration interface { //这个接口也是consul的
	Service //为什么强制要求ServiceRegistration实现Service?
	//其实也是因为consul的agent要求的,这里也是个小接口的原则吧,也有实现了service但没有实现sr
	PublicAddress() string
	PublicPort() int
	DialURL() string
	GetTags() []string
	GetProtocol() string
	GetNamespace() string
	String() string
}

// Server 定义了一个tcp/websocket不同协议通用的服务端的接口
//区分,这里是server,是服务者,不是service,和服务!=
type Server interface {
	// ServiceRegistration 这里是个接口嵌入接口，所有实现Server接口的都还要实现这个naming.SR 接口
	//而在具体的NewServer实现之中,我们要求使用者传入一个实现了SR接口的service参数,然后直接作为这个的字段

	ServiceRegistration

	// SetAcceptor 设置Acceptor
	SetAcceptor(Acceptor)
	//SetMessageListener 设置上行消息监听器
	SetMessageListener(MessageListener)
	//SetStateListener 设置连接状态监听服务
	SetStateListener(StateListener)
	// SetReadWait 设置读超时
	SetReadWait(time.Duration)
	// SetChannelMap ChannelMap 设置Channel管理服务
	SetChannelMap(ChannelMap)

	// Start 用于在内部实现网络端口的监听和接收连接，
	// 并完成一个Channel的初始化过程。
	Start() error
	// Push Push消息到指定的Channel中
	// 	string channelID
	// 	[]byte 序列化之后的消息数据
	Push(string, []byte) error
	// Shutdown 服务下线，关闭连接
	Shutdown(context.Context) error
}

// Acceptor 连接接收器
type Acceptor interface {
	// Accept 返回一个握手完成的Channel对象或者一个error。
	// 业务层需要处理不同协议和网络环境的下连接握手协议
	Accept(Conn, time.Duration) (string, error)
}

// MessageListener 监听消息
type MessageListener interface {
	// 收到消息回调
	Receive(Agent, []byte)
}

// StateListener 状态监听器
type StateListener interface {
	// 连接断开回调
	Disconnect(string) error
}

// Agent is interface of client side
type Agent interface {
	ID() string
	Push([]byte) error
}

// Conn Connection,conn接口,主要由读写frame功能.
//嵌入net.Conn接口强制实现这个接口
type Conn interface {
	net.Conn
	ReadFrame() (Frame, error)
	WriteFrame(OpCode, []byte) error
	Flush() error
}

// Channel is interface of client side
//Channel接口,这玩意和channelMap不同,这个是对于单个链接,那个是针对整个连接
//也就是这个是client side,另一个channelMap是服务侧.
type Channel interface {
	Conn
	Agent
	// Close 关闭连接
	Close() error
	Readloop(lst MessageListener) error
	// SetWriteWait 设置写超时
	SetWriteWait(time.Duration)
	SetReadWait(time.Duration)
}

//channel和client两个接口实际上有些重叠,但是他们呢各有侧重

// Client is interface of client side
//这个是client的抽象接口,所有客户端都要实现这个接口,比如说tcp的客户端.ws的客户端
type Client interface {
	Service
	// connect to server
	Connect(string) error
	// SetDialer 设置拨号处理器
	SetDialer(Dialer)
	Send([]byte) error
	Read() (Frame, error)
	// Close 关闭
	Close()
}

// Dialer 拨号者,该接口一个拨号&握手的方法
type Dialer interface {
	//这个接口的方法用来服务之间建立连接:比如说'聊天服务'和'网关服务'之间的连接,就需要一个TcpDialer去建立握手.
	DialAndHandshake(DialerContext) (net.Conn, error)
}

type DialerContext struct {
	Id      string
	Name    string
	Address string
	Timeout time.Duration
}

// OpCode OpCode
type OpCode byte

// Opcode type
const (
	OpContinuation OpCode = 0x0
	OpText         OpCode = 0x1
	OpBinary       OpCode = 0x2
	OpClose        OpCode = 0x8
	OpPing         OpCode = 0x9
	OpPong         OpCode = 0xa
)

// Frame Frame
type Frame interface {
	SetOpCode(OpCode)
	GetOpCode() OpCode
	SetPayload([]byte)
	GetPayload() []byte
}
