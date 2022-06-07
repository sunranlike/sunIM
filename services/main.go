package main

import (
	"context"
	"flag"
	"github.com/sunrnalike/sun/logger"
	"github.com/sunrnalike/sun/services/gateway"
	"github.com/sunrnalike/sun/services/server"

	"github.com/spf13/cobra"
)

const version = "v1"

func main() {
	flag.Parse()

	root := &cobra.Command{
		Use:     "sun",
		Version: version,
		Short:   "King IM Cloud",
	}
	ctx := context.Background()

	root.AddCommand(gateway.NewServerStartCmd(ctx, version))
	root.AddCommand(server.NewServerStartCmd(ctx, version))

	if err := root.Execute(); err != nil {
		logger.WithError(err).Fatal("Could not run command")
	}
}
