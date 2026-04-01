package config

type KafkaConfig struct {
	Brokers  []string            `mapstructure:"brokers"`
	Consumer KafkaConsumerConfig `mapstructure:"consumer"`
	Producer KafkaProducerConfig `mapstructure:"producer"`
}

type KafkaConsumerConfig struct {
	Group  string `mapstructure:"group"`
	Topic  string `mapstructure:"topic"`
	Offset string `mapstructure:"offset"`
}

type KafkaProducerConfig struct {
	Topic string `mapstructure:"topic"`
}
