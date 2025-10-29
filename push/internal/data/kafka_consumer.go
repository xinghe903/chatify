package data

import (
	"context"
	"push/internal/biz"
	"push/internal/conf"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/IBM/sarama"
)

const (
	KafkaTopicUserState = "user_state"
)

var _ biz.Consumer = (*kafkaConsumer)(nil)

type kafkaConsumer struct {
	consumerGroup sarama.ConsumerGroup
	log           *log.Helper
	topics        []string
}

func NewKafkaConsumer(c *conf.Bootstrap, logger log.Logger) (biz.Consumer, func()) {
	kconf := c.Data.Kafka
	logg := log.NewHelper(logger)
	// 创建 Sarama 配置
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange // 分区分配策略
	config.Consumer.Offsets.Initial = sarama.OffsetOldest                  // 从最旧消息开始消费
	config.Consumer.Return.Errors = true                                   // 启用错误通道

	// 可选：启用自动提交 offset
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

	// 创建消费者组
	consumerGroup, err := sarama.NewConsumerGroup(kconf.Brokers, kconf.GroupId, config)
	if err != nil {
		panic("创建消费者组失败" + err.Error())
	}
	cleanup := func() {
		if err := consumerGroup.Close(); err != nil {
			logg.Errorf("关闭消费者组失败: %v", err)
		}
	}
	return &kafkaConsumer{
		consumerGroup: consumerGroup,
		log:           logg,
		topics:        []string{KafkaTopicUserState},
	}, cleanup
}

func (k *kafkaConsumer) Start(ctx context.Context, handler biz.MessageHandler) {
	k.log.Info("消费者组启动，等待消息...")
	// 启动消费者
	go func() {
		handler := consumerGroupHandler{
			handler: handler,
			log:     k.log,
		}
		for {
			// 消费消息
			if err := k.consumerGroup.Consume(ctx, k.topics, handler); err != nil {
				k.log.Errorf("consumer handler error: %v")
				continue
			}
			// 如果 context 被取消，退出循环
			if ctx.Err() != nil {
				return
			}
			// 重试机制：等待一小段时间后重新连接
			time.Sleep(2 * time.Second)
		}
	}()

}

// 消费者组处理器
type consumerGroupHandler struct {
	log     *log.Helper
	handler biz.MessageHandler
}

// Setup 在消费者加入组后、开始消费前调用
func (h consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup 在消费者离开组前调用
func (h consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 处理每个分区的消息
func (h consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		h.log.Debugf("收到消息: Topic=%s, Partition=%d, Offset=%d, Key=%s, Value=%s, Timestamp=%v",
			message.Topic,
			message.Partition,
			message.Offset,
			string(message.Key),
			string(message.Value),
			message.Timestamp,
		)
		if err := h.handler(context.TODO(), string(message.Key), message.Value); err != nil {
			h.log.Errorf("处理消息失败: %v", err)
		}
		// 手动提交位移（可选，也可以设置自动提交）
		session.MarkMessage(message, "")
	}
	return nil
}
