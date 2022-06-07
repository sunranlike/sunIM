package consul

import (
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/naming"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Naming(t *testing.T) {
	ns, err := NewNaming("localhost:8500") //创建一个服务注册
	assert.Nil(t, err)
	//Nil asserts that the specified object is nil.
	//t是一个testing.T变量,
	//如何做测试?就是把可能的err传给assert.Nil,判断err是否为空,第一个参数都要是t,这是一个类似context的含义.
	//如果err为空,这测试才会通过.
	//如果我是要assert.Equal(t,A,B)
	//就会判断A和B是否相同,如果不相同这个测试就不通过
	//既断言结果不成立---->测试不通过
	//可以说testfy就是通过断言来判断是否通过的
	assert.Equal(t, err, 1, "they should be Equal")

	// 准备工作
	_ = ns.Deregister("test_1")
	_ = ns.Deregister("test_2") //先取消注册

	serviceName := "for_test"
	// 1. 注册 test_1
	err = ns.Register(&naming.DefaultService{
		Id:        "test_1",
		Name:      serviceName,
		Namespace: "",
		Address:   "localhost",
		Port:      8000,
		Protocol:  "ws",
		Tags:      []string{"tab1", "gate"},
	})
	assert.Nil(t, err)

	// 2. 服务发现
	servs, err := ns.Find(serviceName)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(servs))
	t.Log(servs)

	wg := sync.WaitGroup{}
	wg.Add(1)

	// 3. 监听服务实时变化（新增）
	_ = ns.Subscribe(serviceName, func(services []sun.ServiceRegistration) {
		t.Log(len(services))

		assert.Equal(t, 2, len(services))
		assert.Equal(t, "test_2", services[1].ServiceID())
		wg.Done()
	})
	time.Sleep(time.Second)

	// 4. 注册 test_2 用于验证第3步
	err = ns.Register(&naming.DefaultService{
		Id:        "test_2",
		Name:      serviceName,
		Namespace: "",
		Address:   "localhost",
		Port:      8001,
		Protocol:  "ws",
		Tags:      []string{"tab2", "gate"},
	})
	assert.Nil(t, err)

	// 等 Watch 回调中的方法执行完成
	wg.Wait()

	_ = ns.Unsubscribe(serviceName)

	// 5. 服务发现
	servs, _ = ns.Find(serviceName, "gate")
	assert.Equal(t, 2, len(servs)) // <-- 必须有两个

	// 6. 服务发现, 验证tag查询
	servs, _ = ns.Find(serviceName, "tab2")
	assert.Equal(t, 1, len(servs)) // <-- 必须有1个
	assert.Equal(t, "test_2", servs[0].ServiceID())

	// 7. 注销test_2
	err = ns.Deregister("test_2")
	assert.Nil(t, err)

	// 8. 服务发现
	servs, err = ns.Find(serviceName)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(servs))
	assert.Equal(t, "test_1", servs[0].ServiceID())

	// 9. 注销test_1
	err = ns.Deregister("test_1")
	assert.Nil(t, err)

}
