package main

import (
	"flag"
	"os"

	"access/internal/conf"

	"pkg/monitoring"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string = "access"
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		// kratos.ID(id), // 服务ID 暂时用系统生成的，生产环境可以用kratos默认的hostname
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Registrar(r),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}
	tracingConf := bc.Monitoring.Tracing
	var endpoint string
	if tracingConf.Exporter == "jaeger" {
		endpoint = tracingConf.Jaeger.Endpoint
	}
	monitoring.InitTraceProvider(endpoint, bc.Monitoring.ServiceName, tracingConf.Exporter, tracingConf.Sampler)
	loggingConf := bc.Monitoring.Logging
	// 初始化zap日志器
	zapLogger := monitoring.InitLogger(&monitoring.LoggingConfig{
		Format: loggingConf.Format,
		Level:  loggingConf.Level,
		Output: loggingConf.Output,
	})
	svrInstance := conf.ServerInstance{Name: Name, Version: Version, Metadata: map[string]string{}}
	app, cleanup, err := wireApp(&bc, zapLogger, &svrInstance)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	svrInstance.Id = app.ID()
	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
