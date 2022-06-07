package consul

import (
	"errors"
	"fmt"
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

const (
	HealthPassing  = "passing"
	HealthWarning  = "warning"
	HealthCritical = "critical"
	HealthMaint    = "maintenance"
)

//在Check请示时如果服务返回2xx的状态码表示服务正常，状态标记为passing。
//在Check请示时如果服务返回429的状态码表示服务压力大，状态标记为warning。
//其它情况下表示服务故障，状态标记为critical。
//maintenance指服务处理维护状态，通过接口手动设置。

//本文件就是对consul的一些接口进行包装,最主要的是register方法
const (
	KeyProtocol  = "protocol"
	KeyHealthURL = "health_url"
)

type Watch struct {
	Service   string
	Callback  func([]sun.ServiceRegistration)
	WaitIndex uint64
	Quit      chan struct{}
}

//定义Naming类并且提供New方法.我的这个Naming是不是也可以用一个naming?用一个字段是小写的?
//好像不行,因为有一个包就是naming的名字,那改名叫namingImpl得了
type namingImpl struct { //该结构体实现了Naming接口
	sync.RWMutex
	cli    *api.Client
	watchs map[string]*Watch
}

//实现一个New构造函数,我们在cobra真正要调用的就是这个,
//当然测试用例也会调用这个去测试
func NewNaming(consulUrl string) (naming.Naming, error) {
	conf := api.DefaultConfig() //初始化consul的conf
	conf.Address = consulUrl
	cli, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}
	naming := &namingImpl{
		cli:    cli,
		watchs: make(map[string]*Watch, 1),
	}

	return naming, nil
}

func (n *namingImpl) Find(name string, tags ...string) ([]sun.ServiceRegistration, error) { //服务发现,低层还用的是load方法
	services, _, err := n.load(name, 0, tags...)
	if err != nil {
		return nil, err
	}
	return services, nil
}

// refresh service registration
//主要逻辑在n.load方法中；其中第二个参数是waitIndex，用于阻塞查询，主要在Watch时被使用，这里传0表示不阻塞。
func (n *namingImpl) load(name string, waitIndex uint64, tags ...string) ([]sun.ServiceRegistration, *api.QueryMeta, error) {
	opts := &api.QueryOptions{ //查找选项,默认用cache
		UseCache:  true,
		MaxAge:    time.Minute, // 缓冲时间最大限制
		WaitIndex: waitIndex,   //这个用来表明是不是阻塞查询
	}
	catalogServices, meta, err := n.cli.Catalog().ServiceMultipleTags(name, tags, opts) //这个才是真正的寻找name&tag对应的服务
	if err != nil {
		return nil, meta, err
	}

	services := make([]sun.ServiceRegistration, len(catalogServices)) //写一个服务的slice
	for i, s := range catalogServices {                               //遍历刚刚的从consul查到的service然后传入service[i]中
		if s.Checks.AggregatedStatus() != api.HealthPassing { //遍历中还要做服务检查是否健康
			logger.Debugf("load service: id:%s name:%s %s:%d Status:%s", s.ServiceID, s.ServiceName, s.ServiceAddress, s.ServicePort, s.Checks.AggregatedStatus())
			continue
		}
		services[i] = &naming.DefaultService{ //将这个s的id,name,address地址,端口都传入这个slice中保存下来返回
			Id:       s.ServiceID,
			Name:     s.ServiceName,
			Address:  s.ServiceAddress,
			Port:     s.ServicePort,
			Protocol: s.ServiceMeta[KeyProtocol],
			Tags:     s.ServiceTags,
			Meta:     s.ServiceMeta,
		}
	}
	logger.Debugf("load service: %v, meta:%v", services, meta)
	return services, meta, nil
}

//Register: 传入的是一个SR,这个接口规定的参数列表,都在consul.api.AgentServiceRegistration中有定义
//这样直接对应字段赋值就好了
func (n *namingImpl) Register(s sun.ServiceRegistration) error {
	reg := &api.AgentServiceRegistration{ //将我的service的参数赋值给consul的注册
		ID:      s.ServiceID(),
		Name:    s.ServiceName(),
		Address: s.PublicAddress(),
		Port:    s.PublicPort(),
		Tags:    s.GetTags(),
		Meta:    s.GetMeta(),
	}
	if reg.Meta == nil { //
		reg.Meta = make(map[string]string)
	}
	reg.Meta[KeyProtocol] = s.GetProtocol() //沟通协议，告诉reg我的具体协议

	// consul健康检查
	healthURL := s.GetMeta()[KeyHealthURL]
	if healthURL != "" {
		check := new(api.AgentServiceCheck)
		check.CheckID = fmt.Sprintf("%s_normal", s.ServiceID())
		check.HTTP = healthURL
		check.Timeout = "1s" // http timeout
		check.Interval = "10s"
		check.DeregisterCriticalServiceAfter = "20s"
		reg.Check = check
	}

	err := n.cli.Agent().ServiceRegister(reg) //最后真正的注册,在本地agent注册服务
	return err
}

func (n *namingImpl) Deregister(serviceID string) error {
	return n.cli.Agent().ServiceDeregister(serviceID)
}

func (n *namingImpl) Unsubscribe(serviceName string) error {
	n.Lock()
	defer n.Unlock()
	wh, ok := n.watchs[serviceName]

	delete(n.watchs, serviceName)
	if ok {
		close(wh.Quit)
	}
	return nil
}

//这里我们创建了一个Watch对象，并且make了有一个缓冲位的chan Quit，它会被使用到Unsubscribe中。
//而订阅真下执行的逻辑在go n.watch(w)中，我们启用了一个新的线程执行监听任务。
//在consul的API中 ，支持阻塞式调用的，
//即响应并不会立即返回，直到有变化或wait时间到，也就是我们常说的长轮询long-polling，
//比如前面提到的Nacos也是采用同样的方式来感知服务变化的。
func (n *namingImpl) Subscribe(serviceName string, callback func([]sun.ServiceRegistration)) error {
	n.Lock()
	defer n.Unlock()
	//订阅为什么先加锁?因为watchs map有并发操作的风险,要加锁.
	if _, ok := n.watchs[serviceName]; ok {
		//如果你要订阅的服务已经在watchs map中了,那你还从服务容器中注册个p啊
		//都已经有了. error!
		return errors.New("serviceName has already been registered")
	}
	w := &Watch{ //如果说我要wathc的服务不在wathcs之中:需要将这个service添加到我的wathcs中
		Service:  serviceName,
		Callback: callback,
		Quit:     make(chan struct{}, 1),
	}
	n.watchs[serviceName] = w //将这个添加到watchs之中,代表已经监听了,用来帮助不要重发wathc一个

	go n.watch(w) //开启一个routine去监听这个服务
	return nil
}

func (n *namingImpl) watch(wh *Watch) { //该方法就是用来监听的.低层用的还是load,load低层还是用的consul的api
	stopped := false

	var doWatch = func(service string, callback func([]sun.ServiceRegistration)) {
		//watch低层调用的也是load,并且是会阻塞的
		services, meta, err := n.load(service, wh.WaitIndex) // <-- blocking untill services has changed
		if err != nil {
			logger.Warn(err)
			return
		}
		select {
		case <-wh.Quit:
			stopped = true
			logger.Infof("watch %s stopped", wh.Service)
			return
		default:
		}

		wh.WaitIndex = meta.LastIndex
		if callback != nil {
			callback(services)
		}
	}

	// build WaitIndex
	doWatch(wh.Service, nil)
	for !stopped {
		//subscribe会开一个routine开启wathc,然后这个routine在这里会无限重复地调用,
		//这个阻塞调用会直到调用unsubscribe方法,使得 wh.Quit这个chan被关闭,导致return,结束dowatch,
		doWatch(wh.Service, wh.Callback)
	}
}
