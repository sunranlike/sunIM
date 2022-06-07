package serv

import (
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/tcp"
	"github.com/sunrnalike/sun/wire/pkt"
	"net"

	"google.golang.org/protobuf/proto"
)

type TcpDialer struct { //TcpDialer实现了dialer接口,这个接口只有一个方法就是拨号与握手
	//实际上拨号者做的就是DialAndHandshake
	ServiceId string
}

func NewDialer(serviceId string) sun.Dialer {
	return &TcpDialer{
		ServiceId: serviceId,
	}
}

// DialAndHandshake(context.Context, string) (net.Conn, error)
//这个方法帮助tcpDialer实现Dialer接口,我们的网关需要使用tcp协议建立连接，
func (d *TcpDialer) DialAndHandshake(ctx sun.DialerContext) (net.Conn, error) {
	// 1. 拨号建立连接,调用官方net库,conn这个参数是连接,我们对这个进行writeFrame操作
	conn, err := net.DialTimeout("tcp", ctx.Address, ctx.Timeout)
	if err != nil {
		return nil, err
	}
	req := &pkt.InnerHandshakeReq{ //request,谁调用DialAndHandshake谁就会有一个ServiceId
		//我们的req就是存放这个id的一个结构,因为这个方法基本只有内网使用,基于内网可靠性设计的,所以我们并无
		//任何权限处理,只需要指导serviceId即可

		ServiceId: d.ServiceId,
	}
	logger.Infof("send req %v", req) //打印req
	// 2. 把自己的ServiceId发送给对方.
	//使用protobuf进行序列化,将req写入bts,bts应该是binary ts的意思?
	bts, _ := proto.Marshal(req) //bts存放的的是:InnerHandshakeReq结构体,其中有一个id

	//这里是最终写数据的逻辑:第一个参数是conn,第二个参数是代表是二进制数据,第三个参数就是bts(有id)
	//这样就能够告诉conn的对象我的id
	err = tcp.WriteFrame(conn, sun.OpBinary, bts)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
