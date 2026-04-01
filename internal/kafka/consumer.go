package kafka

import (
	"github.com/IBM/sarama"
)

type Consumer struct {
	sarama.Consumer
	group sarama.ConsumerGroup
}

func NewConsumer(brokers []string, group string) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}

	consumer, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		return nil, err
	}

	return &Consumer{group: consumer}, nil
}
