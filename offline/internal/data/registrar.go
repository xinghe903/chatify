package data

import (
	"offline-message/internal/conf"
	registryetcd "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// NewEtcdClient 创建etcd客户端
func NewEtcdClient(cb *conf.Bootstrap) (*clientv3.Client, error) {
	c := cb.Data.Etcd
	config := clientv3.Config{
		Endpoints:   c.Endpoints,
		DialTimeout: c.DialTimeout.AsDuration(),
	}

	if c.Username != "" && c.Password != "" {
		config.Username = c.Username
		config.Password = c.Password
	}

	return clientv3.New(config)
}

// NewRegistry 创建服务注册器
func NewRegistry(etcd *clientv3.Client) registry.Registrar {
	return registryetcd.New(etcd)
}