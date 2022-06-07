package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"
	"github.com/sunrnalike/sun/tcp"
	"github.com/sunrnalike/sun/wire"
	"github.com/sunrnalike/sun/wire/pkt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	stateUninitialized = iota
	stateInitialized
	stateStarted
	stateClosed
)

const (
	StateYoung = "young"
	StateAdult = "adult"
)

const (
	KeyServiceState = "service_state"
)

// Container Container
type Container struct {
	sync.RWMutex
	Naming     naming.Naming
	Srv        sun.Server
	state      uint32
	srvclients map[string]ClientMap
	selector   Selector
	dialer     sun.Dialer
	deps       map[string]struct{}
	monitor    sync.Once
}

var log = logger.WithField("module", "container")

// Default Container
var defaultContainer = &Container{
	state:    0,
	selector: &HashSelector{},
	deps:     make(map[string]struct{}),
}

// Default Default
func Default() *Container {
	return defaultContainer
}

// Init Init
//Init目的就是将参数srv传入到container中,这个方法是Star()前必须调用的,否则也就没有意义了:服务容器没有服务,那你还干嘛
//比如说我们的defaultContainer,的srv字段没有赋任何值,而start中到处都是对srv的调用,
//我们需要做的就是调用init,传入一下服务,这样start才有的srv可以调用
func Init(srv sun.Server, deps ...string) error {
	if !atomic.CompareAndSwapUint32(&defaultContainer.state, stateUninitialized, stateInitialized) {
		return errors.New("has Initialized")
	}
	defaultContainer.Srv = srv
	for _, dep := range deps { //deps是一个变长参数字段,这个应当是
		if _, ok := defaultContainer.deps[dep]; ok {
			continue
		}
		defaultContainer.deps[dep] = struct{}{}
	}
	log.WithField("func", "Init").Infof("srv %s:%s - deps %v", srv.ServiceID(), srv.ServiceName(), defaultContainer.deps)
	defaultContainer.srvclients = make(map[string]ClientMap, len(deps))
	return nil
}

// SetDialer set tcp dialer
func SetDialer(dialer sun.Dialer) {
	defaultContainer.dialer = dialer
}

/// EnableMonitor start
func EnableMonitor(listen string) error {
	return nil
}

// SetSelector set a default selector
func SetSelector(selector Selector) {
	defaultContainer.selector = selector
}

// SetServiceNaming
func SetServiceNaming(nm naming.Naming) {
	defaultContainer.Naming = nm
}

// Start server
//启动的前提是注册号,也就是init好,需要哪些服务,那些依赖服务deps
func Start() error {
	if defaultContainer.Naming == nil {
		return fmt.Errorf("naming is nil")
	}

	if !atomic.CompareAndSwapUint32(&defaultContainer.state, stateInitialized, stateStarted) {
		return errors.New("has started")
	}

	// 1. 启动container.Server,选择了开启一个goroutine的方式.
	go func(srv sun.Server) {
		err := srv.Start()
		if err != nil {
			log.Errorln(err)
		}
	}(defaultContainer.Srv) //启动container的server.
	//defaultContainer.srv是一个Server接口的具体化实现,而具体调用则会被低层的server具体实现决定.

	// 2. 与依赖的服务建立连接.我的c.srv会有大量的依赖服务存在。
	for service := range defaultContainer.deps {
		//deps相当于依赖,这里会遍历deps,然后跟deps这个string对应的服务建立连接
		go func(service string) {
			err := connectToService(service)
			if err != nil {
				log.Errorln(err)
			}
		}(service)
	}

	//3. 服务注册，调用register服务，向container中注册服务
	if defaultContainer.Srv.PublicAddress() != "" && defaultContainer.Srv.PublicPort() != 0 {
		err := defaultContainer.Naming.Register(defaultContainer.Srv) //向容器中注册服务,
		// 也可以看到我们不停地使用Srv字段,但是这个字段是一个接口,还没有赋予具体的服务,所以必须要调用init传入srv才可以
		if err != nil {
			log.Errorln(err)
		}
	}

	// wait quit signal of system
	//也可以看到这个是整个container的服务周期
	c := make(chan os.Signal, 1)
	//只有这些信号来了才会退出,才会继续往下走
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Infoln("shutdown", <-c)
	// 4. 退出
	return shutdown()
}

// push message to server
func Push(server string, p *pkt.LogicPkt) error {
	p.AddStringMeta(wire.MetaDestServer, server)
	return defaultContainer.Srv.Push(server, pkt.Marshal(p))
}

// Forward message to service
func Forward(serviceName string, packet *pkt.LogicPkt) error {
	if packet == nil {
		return errors.New("packet is nil")
	}
	if packet.Command == "" {
		return errors.New("command is empty in packet")
	}
	if packet.ChannelId == "" {
		return errors.New("ChannelId is empty in packet")
	}
	return ForwardWithSelector(serviceName, packet, defaultContainer.selector)
}

// ForwardWithSelector forward data to the specified node of service which is chosen by selector
func ForwardWithSelector(serviceName string, packet *pkt.LogicPkt, selector Selector) error {
	cli, err := lookup(serviceName, &packet.Header, selector)
	if err != nil {
		return err
	}
	// add a tag in packet
	packet.AddStringMeta(wire.MetaDestServer, defaultContainer.Srv.ServiceID())
	log.Debugf("forward message to %v with %s", cli.ServiceID(), &packet.Header)
	return cli.Send(pkt.Marshal(packet))
}

// shutdown Shutdown
func shutdown() error {
	if !atomic.CompareAndSwapUint32(&defaultContainer.state, stateStarted, stateClosed) {
		return errors.New("has closed")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()
	// 1. 优雅关闭服务器
	err := defaultContainer.Srv.Shutdown(ctx)
	if err != nil {
		log.Error(err)
	}
	// 2. 从注册中心注销服务
	err = defaultContainer.Naming.Deregister(defaultContainer.Srv.ServiceID())
	if err != nil {
		log.Warn(err)
	}
	// 3. 退订服务变更
	for dep := range defaultContainer.deps {
		_ = defaultContainer.Naming.Unsubscribe(dep)
	}

	log.Infoln("shutdown")
	return nil
}

func lookup(serviceName string, header *pkt.Header, selector Selector) (sun.Client, error) {
	clients, ok := defaultContainer.srvclients[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}
	// 只获取状态为StateAdult的服务
	srvs := clients.Services(KeyServiceState, StateAdult)
	if len(srvs) == 0 {
		return nil, fmt.Errorf("no services found for %s", serviceName)
	}
	id := selector.Lookup(header, srvs)
	if cli, ok := clients.Get(id); ok {
		return cli, nil
	}
	return nil, fmt.Errorf("no client found")
}

func connectToService(serviceName string) error {
	clients := NewClients(10)
	defaultContainer.srvclients[serviceName] = clients
	// 1. 首先Watch服务的新增
	delay := time.Second * 10
	err := defaultContainer.Naming.Subscribe(serviceName, func(services []sun.ServiceRegistration) {
		for _, service := range services {
			if _, ok := clients.Get(service.ServiceID()); ok {
				continue
			}
			log.WithField("func", "connectToService").Infof("Watch a new service: %v", service)
			service.GetMeta()[KeyServiceState] = StateYoung
			go func(service sun.ServiceRegistration) {
				time.Sleep(delay)
				service.GetMeta()[KeyServiceState] = StateAdult
			}(service)

			_, err := buildClient(clients, service)
			if err != nil {
				logger.Warn(err)
			}
		}
	})
	if err != nil {
		return err
	}
	// 2. 再查询已经存在的服务
	services, err := defaultContainer.Naming.Find(serviceName)
	if err != nil {
		return err
	}
	log.Info("find service ", services)
	for _, service := range services {
		// 标记为StateAdult
		service.GetMeta()[KeyServiceState] = StateAdult
		_, err := buildClient(clients, service)
		if err != nil {
			logger.Warn(err)
		}
	}
	return nil
}

func buildClient(clients ClientMap, service sun.ServiceRegistration) (sun.Client, error) {
	defaultContainer.Lock()
	defer defaultContainer.Unlock()
	var (
		id   = service.ServiceID()
		name = service.ServiceName()
		meta = service.GetMeta()
	)
	// 1. 检测连接是否已经存在
	if _, ok := clients.Get(id); ok {
		return nil, nil
	}
	// 2. 服务之间只允许使用tcp协议
	if service.GetProtocol() != string(wire.ProtocolTCP) {
		return nil, fmt.Errorf("unexpected service Protocol: %s", service.GetProtocol())
	}

	// 3. 构建客户端并建立连接
	cli := tcp.NewClientWithProps(id, name, meta, tcp.ClientOptions{
		Heartbeat: sun.DefaultHeartbeat,
		ReadWait:  sun.DefaultReadWait,
		WriteWait: sun.DefaultWriteWait,
	})
	if defaultContainer.dialer == nil {
		return nil, fmt.Errorf("dialer is nil")
	}
	cli.SetDialer(defaultContainer.dialer)
	err := cli.Connect(service.DialURL())
	if err != nil {
		return nil, err
	}
	// 4. 读取消息
	go func(cli sun.Client) {
		err := readLoop(cli)
		if err != nil {
			log.Debug(err)
		}
		clients.Remove(id)
		cli.Close()
	}(cli)
	// 5. 添加到客户端集合中
	clients.Add(cli)
	return cli, nil
}

// Receive default listener
func readLoop(cli sun.Client) error {
	log := logger.WithFields(logger.Fields{
		"module": "container",
		"func":   "readLoop",
	})
	log.Infof("readLoop started of %s %s", cli.ServiceID(), cli.ServiceName())
	for {
		frame, err := cli.Read()
		if err != nil {
			return err
		}
		if frame.GetOpCode() != sun.OpBinary {
			continue
		}
		buf := bytes.NewBuffer(frame.GetPayload())

		packet, err := pkt.MustReadLogicPkt(buf)
		if err != nil {
			log.Info(err)
			continue
		}
		err = pushMessage(packet)
		if err != nil {
			log.Info(err)
		}
	}
}

// 消息通过网关服务器推送到channel中
func pushMessage(packet *pkt.LogicPkt) error {
	server, _ := packet.GetMeta(wire.MetaDestServer)
	if server != defaultContainer.Srv.ServiceID() {
		return fmt.Errorf("dest_server is not incorrect, %s != %s", server, defaultContainer.Srv.ServiceID())
	}
	channels, ok := packet.GetMeta(wire.MetaDestChannels)
	if !ok {
		return fmt.Errorf("dest_channels is nil")
	}

	channelIds := strings.Split(channels.(string), ",")
	packet.DelMeta(wire.MetaDestServer)
	packet.DelMeta(wire.MetaDestChannels)
	payload := pkt.Marshal(packet)
	log.Debugf("Push to %v %v", channelIds, packet)

	for _, channel := range channelIds {
		err := defaultContainer.Srv.Push(channel, payload)
		if err != nil {
			log.Debug(err)
		}
	}
	return nil
}
