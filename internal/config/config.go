package config

import (
	"github.com/cristalhq/aconfig"
)

type Snapshot struct {
	Disable bool   `env:"DISABLE"`
	Path    string `env:"PATH"`
}

type Config struct {
	Debug          bool     `env:"DEBUG"`
	Address        string   `env:"ADDRESS" default:":8080"`
	ServiceAddress string   `env:"SERVICE_ADDRESS" default:":8081"`
	Snapshot       Snapshot `envPrefix:"SNAPSHOT"`
}

func Load() *Config {
	cfg := Config{}

	err := aconfig.LoaderFor(&cfg, aconfig.Config{
		EnvPrefix: "SSQUEUE",
	}).Load()

	if err != nil {
		panic(err)
	}

	return &cfg
}
