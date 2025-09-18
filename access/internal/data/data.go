package data

import (
	"access/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData)

// Data .
type Data struct {
	mq *MqConsumer
}

func NewData(c *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	// 初始化Kafka消费者
	kafkaConsumer, err := NewMqConsumer(c.Data)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		kafkaConsumer.Close()
		log.NewHelper(logger).Info("closing the data resources")
	}

	return &Data{
		mq: kafkaConsumer,
	}, cleanup, nil
}

type MqConsumer struct {
}

// NewMqConsumer 创建Kafka消费者实例
func NewMqConsumer(c *conf.Data) (*MqConsumer, error) {

	return &MqConsumer{}, nil
}

// Close 关闭消费者连接
func (kc *MqConsumer) Close() error {
	return nil
}

// GetReader 获取Kafka reader（供biz层调用）
func (kc *MqConsumer) GetReader() {

}
