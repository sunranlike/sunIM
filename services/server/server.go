package server

import (
	"context"
	"github.com/sunrnalike/sun/storage"

	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/container"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/naming"
	"github.com/sunrnalike/sun/naming/consul"
	"github.com/sunrnalike/sun/services/server/conf"
	"github.com/sunrnalike/sun/services/server/handler"
	"github.com/sunrnalike/sun/services/server/serv"
	"github.com/sunrnalike/sun/tcp"
	"github.com/sunrnalike/sun/wire"

	"github.com/spf13/cobra"
)

// ServerStartOptions ServerStartOptions
type ServerStartOptions struct {
	config      string
	serviceName string
}

// NewServerStartCmd creates a new http server command
func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./server/conf.yaml", "Config file")
	cmd.PersistentFlags().StringVarP(&opts.serviceName, "serviceName", "s", "chat", "defined a service name,option is login or chat")
	return cmd
}

// RunServerStart run http server
func RunServerStart(ctx context.Context, opts *ServerStartOptions, version string) error {
	config, err := conf.Init(opts.config) //服务初始化
	if err != nil {
		return err
	}
	_ = logger.Init(logger.Settings{ //logger初始化
		Level: "trace",
	})
	//router处理
	r := sun.NewRouter()
	// login
	loginHandler := handler.NewLoginHandler()
	r.Handle(wire.CommandLoginSignIn, loginHandler.DoSysLogin)
	r.Handle(wire.CommandLoginSignOut, loginHandler.DoSysLogout)
	//新建一个redis客户端
	rdb, err := conf.InitRedis(config.RedisAddrs, "")
	if err != nil {
		return err
	}
	cache := storage.NewRedisStorage(rdb)
	servhandler := serv.NewServHandler(r, cache)

	service := &naming.DefaultService{
		Id:       config.ServiceID,
		Name:     opts.serviceName,
		Address:  config.PublicAddress,
		Port:     config.PublicPort,
		Protocol: string(wire.ProtocolTCP),
		Tags:     config.Tags,
	}
	srv := tcp.NewServer(config.Listen, service)

	srv.SetReadWait(sun.DefaultReadWait)
	srv.SetAcceptor(servhandler)
	srv.SetMessageListener(servhandler)
	srv.SetStateListener(servhandler)
	//容器初始化,这里不需要传入什么服务了
	if err := container.Init(srv); err != nil {
		return err
	}
	//naming初始化
	ns, err := consul.NewNaming(config.ConsulURL)
	if err != nil {
		return err
	}
	container.SetServiceNaming(ns)

	return container.Start()
}
