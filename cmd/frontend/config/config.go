package config

type StateStoreConfig struct {
	Name string `mapstructure:"name"`
}

type PubSubConfig struct {
	Name  string `mapstructure:"name"`
	Topic string `mapstructure:"topic"`
	Port  int    `mapstructure:"port"`
}

type Config struct {
	Port       int              `mapstructure:"port"`
	StateStore StateStoreConfig `mapstructure:"statestore"`
	PubSub     PubSubConfig     `mapstructure:"pubsub"`
}
