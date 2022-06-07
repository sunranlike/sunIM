package gateway

import (
	"context"
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/container"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"
	"github.com/sunrnalike/sun/naming/consul"
	"github.com/sunrnalike/sun/services/gateway/conf"
	"github.com/sunrnalike/sun/services/gateway/serv"
	"github.com/sunrnalike/sun/websocket"
	"github.com/sunrnalike/sun/wire"
	"time"

	"github.com/spf13/cobra"
)

// const logName = "logs/gateway"

// ServerStartOptions ServerStartOptions
type ServerStartOptions struct {
	config   string
	protocol string
}

// NewServerStartCmd creates a new http server command
func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Start a gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./gateway/conf.yaml", "Config file")
	cmd.PersistentFlags().StringVarP(&opts.protocol, "protocol", "p", "ws", "protocol of ws or tcp")
	return cmd
}

// RunServerStart run http server
func RunServerStart(ctx context.Context, opts *ServerStartOptions, version string) error {
	config, err := conf.Init(opts.config) //环境变量初始化
	if err != nil {
		return err
	}
	_ = logger.Init(logger.Settings{ //初始化logger
		Level: "trace",
	})

	handler := &serv.Handler{ //handler初始化
		ServiceID: config.ServiceID,
	}

	var srv sun.Server
	service := &naming.DefaultService{
		Id:       config.ServiceID,
		Name:     config.ServiceName,
		Address:  config.PublicAddress,
		Port:     config.PublicPort,
		Protocol: opts.protocol,
		Tags:     config.Tags,
	}
	if opts.protocol == "ws" {
		srv = websocket.NewServer(config.Listen, service)
	}

	srv.SetReadWait(time.Minute * 2)
	srv.SetAcceptor(handler)
	srv.SetMessageListener(handler)
	srv.SetStateListener(handler)

	_ = container.Init(srv, wire.SNChat, wire.SNLogin) //初始化chat和loginin,这里需要传入chat和login

	ns, err := consul.NewNaming(config.ConsulURL) //服务注册初始化
	if err != nil {
		return err
	}
	container.SetServiceNaming(ns)

	// set a dialer
	container.SetDialer(serv.NewDialer(config.ServiceID))

	return container.Start() //启动
}
