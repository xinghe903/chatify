package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"access/internal/biz"
	"access/internal/biz/bo"
	"access/internal/conf"
	"pkg/auth"

	"github.com/IBM/sarama"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.MqProducer = (*KafkaProducer)(nil)

// KafkaProducer Kafka生产者结构体
type KafkaProducer struct {
	producer  sarama.AsyncProducer
	log       *log.Helper
	snowflake *auth.Sonyflake
}

// NewKafkaProducer 创建Kafka生产者实例
func NewKafkaProducer(cb *conf.Bootstrap, logger log.Logger) (biz.MqProducer, func(), error) {
	c := cb.Data.Kafka
	// 配置Sarama生产者
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForLocal // 本地确认
	config.Producer.Retry.Max = int(c.RetryCount)      // 重试次数
	config.Producer.Return.Successes = true            // 返回成功消息
	config.Producer.Return.Errors = true               // 返回错误消息
	config.Producer.Timeout = c.Timeout.AsDuration()   // 超时时间
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	fmt.Printf("Kafka brokers: %v\n", c.Brokers)
	// 创建异步生产者
	producer, err := sarama.NewAsyncProducer(c.Brokers, config)
	if err != nil {
		panic("Failed to create kafka producer")
	}
	// 创建Sonyflake分布式ID生成器
	snowflake := auth.NewSonyflake()
	// 启动goroutine处理成功和错误的消息
	kp := &KafkaProducer{
		producer:  producer,
		log:       log.NewHelper(logger),
		snowflake: snowflake,
	}
	cleanup := func() {
		if err := producer.Close(); err != nil {
			panic("Failed to close kafka producer")
		}
		kp.log.Info("Kafka producer closed")
	}
	// 启动后台 goroutine 扫结果
	go func() {
		for {
			select {
			case suc, ok := <-producer.Successes():
				if !ok {
					kp.log.Infof("Successes channel closed. Exiting monitor goroutine.")
					return
				}
				kp.log.Infof("✅ msg sent offset=%d  partition=%d  topic=%s",
					suc.Offset, suc.Partition, suc.Topic)

			case fail, ok := <-producer.Errors():
				if !ok {
					kp.log.Infof("Errors channel closed. Exiting monitor goroutine.")
					return
				}
				kp.log.Infof("❌ msg err: %v", fail.Err)
			}
		}
	}()
	return kp, cleanup, nil
}

// SendMessage 发送消息到Kafka
func (p *KafkaProducer) SendMessage(ctx context.Context, topic string, message *bo.UserStateMessage) error {
	if p.producer == nil {
		return fmt.Errorf("kafka producer not initialized")
	}
	// 将消息序列化为JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message error: %w", err)
	}
	// 生成消息ID作为Key，用于防止重复消费
	// 使用Sonyflake分布式ID生成器，确保全局唯一
	msgID, err := p.snowflake.GenerateBase62()
	if err != nil {
		p.log.WithContext(ctx).Errorf("Generate message ID error: %v", err)
		return fmt.Errorf("generate message ID error: %w", err)
	}
	// 创建Sarama消息，设置Key为消息ID
	saramaMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(msgID),
		Value: sarama.ByteEncoder(data),
	}
	// 使用select配合context，支持超时和取消
	select {
	case p.producer.Input() <- saramaMsg:
		p.log.WithContext(ctx).Debugf("Send message to kafka success, topic: %s, message: %s", topic, string(data))
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before message could be sent: %w", ctx.Err())
	case <-time.After(5 * time.Second): // 添加发送超时保护
		return fmt.Errorf("timed out sending message to kafka")
	}
}

// Close 关闭Kafka生产者连接
func (p *KafkaProducer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}
