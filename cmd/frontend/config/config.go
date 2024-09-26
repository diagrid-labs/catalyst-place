package config

type StateStoreConfig struct {
	Name string `mapstructure:"name"`
}

type PubSubConfig struct {
	Name  string `mapstructure:"name"`
	Topic string `mapstructure:"topic"`
}

type DiagridConfig struct {
	OrganizationID string        `mapstructure:"organization_id"`
	Endpoint       string        `mapstructure:"endpoint"`
	Project        ProjectConfig `mapstructure:"project"`
}

type ProjectConfig struct {
	Name     string `mapstructure:"name"`
	Frontend string `mapstructure:"frontend"`
}

type CooldownConfig struct {
	Name string `mapstructure:"name"`
	TTL  string `mapstructure:"ttl"`
}

type Config struct {
	Port       int              `mapstructure:"port"`
	Diagrid    DiagridConfig    `mapstructure:"diagrid"`
	StateStore StateStoreConfig `mapstructure:"statestore"`
	PubSub     PubSubConfig     `mapstructure:"pubsub"`
	Cooldown   CooldownConfig   `mapstructure:"cooldown"`
}
