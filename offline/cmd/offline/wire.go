//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/xinghe903/chatify/offline/internal/biz"
	"github.com/xinghe903/chatify/offline/internal/conf"
	"github.com/xinghe903/chatify/offline/internal/data"
	"github.com/xinghe903/chatify/offline/internal/server"
	"github.com/xinghe903/chatify/offline/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
