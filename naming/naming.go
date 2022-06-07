package naming

import (
	"errors"
	"github.com/sunrnalike/sun"
)

// errors
var (
	ErrNotFound = errors.New("service no found")
)

// Naming defined methods of the naming service

//Register - 注册服务
//Deregister - 注销服务
//Find - 服务发现，支持通过tag查询
//Subscribe - 订阅服务变更通知
//Unsubscribe - 取消订阅服务变更通知
type Naming interface { //naming目的是服务注册与发现,起这个名字也是为了可以改用其他去实现服务注册&发现
	Find(serviceName string, tags ...string) ([]sun.ServiceRegistration, error)
	Subscribe(serviceName string, callback func(services []sun.ServiceRegistration)) error
	Unsubscribe(serviceName string) error
	//Naming接口的Register会把srv.ServiceRegistration的一些方法注册到consul之中,从而使用consul
	Register(service sun.ServiceRegistration) error
	Deregister(serviceID string) error
}

//nameing,这个文件是干嘛的!?定义Naming接口!
//why?这个接口如上所示有这些个方法,这也是服务注册发现的关键接口
//1:Register - 注册服务参数是一个ServiceRegistration(SR),SR是什么?
//它是一个服务注册函数,我有类似provider,我规定所有服务都要实现这个接口,就可以注册进服务容器中
//2:Deregister - 注销服务,Register相反
//3:Find:发现服务,支持tag查询
//4:Subscribe - 订阅服务:某某需要订阅xx服务,比如说要订阅缓存服务
//5:Unsubscribe 相反
