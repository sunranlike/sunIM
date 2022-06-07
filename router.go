package sun

import (
	"errors"
	"fmt"
	"github.com/sunrnalike/sun/wire/pkt"
	"sync"
)

var ErrSessionLost = errors.New("err:session lost")

// Router defines
type Router struct {
	middlewares []HandlerFunc //中间件slice
	handlers    *FuncTree     //监听器列表,估计是个二叉树结构
	pool        sync.Pool     //对象池
}

// NewRouter NewRouter
func NewRouter() *Router {
	r := &Router{
		handlers:    NewTree(),
		middlewares: make([]HandlerFunc, 0),
	}
	r.pool.New = func() interface{} { //使用sync.pool对象池必须要设置一个New方法,这个方法是强制的
		//这样才能使用对象池
		return BuildContext() //直接返回BuildContext,这个方法返回一个ContextImpl
	}
	return r
}

func (r *Router) Use(handlers ...HandlerFunc) {
	r.middlewares = append(r.middlewares, handlers...)
}

// Handle register a command handler
func (r *Router) Handle(command string, handlers ...HandlerFunc) {
	r.handlers.Add(command, r.middlewares...)
	r.handlers.Add(command, handlers...)
}

// Serve a packet from client
func (r *Router) Serve(packet *pkt.LogicPkt, dispatcher Dispatcher, cache SessionStorage, session Session) error {
	if dispatcher == nil {
		return fmt.Errorf("dispatcher is nil")
	}
	if cache == nil {
		return fmt.Errorf("cache is nil")
	}
	ctx := r.pool.Get().(*ContextImpl) //从池子中Get这个方法
	ctx.reset()                        //Get到之后reset一下,全部设置为空
	ctx.request = packet
	ctx.Dispatcher = dispatcher
	ctx.SessionStorage = cache
	ctx.session = session //设置参数

	r.serveContext(ctx) //调用serveCtx服务这个ctx
	// Put Context to Pool
	r.pool.Put(ctx) //放入池子中
	return nil
}

func (r *Router) serveContext(ctx *ContextImpl) {
	chain, ok := r.handlers.Get(ctx.Header().Command) //
	if !ok {                                          //如果没有查找到,就告诉没有
		ctx.handlers = []HandlerFunc{handleNoFound}
		ctx.Next()
		return
	}
	ctx.handlers = chain //链式调用
	ctx.Next()
}

func handleNoFound(ctx Context) {
	_ = ctx.Resp(pkt.Status_NotImplemented, &pkt.ErrorResp{Message: "NotImplemented"})
}

// FuncTree is a tree structure
type FuncTree struct {
	nodes map[string]HandlersChain
}

// NewTree NewTree
func NewTree() *FuncTree {
	return &FuncTree{nodes: make(map[string]HandlersChain, 10)}
}

// Add a handler to tree
func (t *FuncTree) Add(path string, handlers ...HandlerFunc) {
	if t.nodes[path] == nil {
		t.nodes[path] = HandlersChain{}
	}

	t.nodes[path] = append(t.nodes[path], handlers...)
}

// Get a handler from tree
func (t *FuncTree) Get(path string) (HandlersChain, bool) {
	f, ok := t.nodes[path]
	return f, ok
}