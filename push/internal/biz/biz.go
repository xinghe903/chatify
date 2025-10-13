package biz

import (
	accessV1 "api/access/v1"
	"push/internal/data"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewPush,
	wire.Bind(new(UserRepo), new(*data.userRepo)),
	wire.Bind(new(MessageRepo), new(*data.messageRepo)),
	wire.Bind(new(accessV1.AccessServiceClient), new(accessV1.AccessServiceClient)),
	wire.Bind(new(*redis.Client), new(*redis.Client)),
	wire.Bind(new(registry.Discovery), new(registry.Discovery)),
)
