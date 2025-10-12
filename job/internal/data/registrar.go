package data

import (
	"time"

	"go.etcd.io/etcd/client/v3"

	"job/internal/conf"

	etcd "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// NewEtcdClient 创建etcd客户端
func NewEtcdClient(conf *conf.Data, logger log.Logger) (*clientv3.Client, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.Etcd.Endpoints,
		DialTimeout: time.Duration(conf.Etcd.DialTimeout.AsDuration()),
		Username:    conf.Etcd.Username,
		Password:    conf.Etcd.Password,
	})
	if err != nil {
		log.NewHelper(logger).Errorw("failed to create etcd client", "err", err)
		return nil, err
	}
	log.NewHelper(logger).Info("etcd client created successfully")
	return client, nil
}

// NewRegistry 创建注册器
func NewRegistry(client *clientv3.Client) etcd.Registrar {
	return etcd.New(client)
}