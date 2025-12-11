package config

import (
	"github.com/vitistack/gslb-operator/pkg/loaders"
)

type Config struct {
	Server Server
	API    API
	GSLB   GSLB
}

// Server configuration
type Server struct {
	Environment string `env:"SRV_ENV" flag:"env"`
}

// API configuration
type API struct {
	Port string `env:"API_PORT" flag:"port"`
}

// GSLB configuration
type GSLB struct {
	Zone       string `env:"GSLB_ZONE" flag:"gslb-zone"`
	NameServer string `env:"GSLB_NAMESERVER" flag:"gslb-nameserver"`
}

func NewConfig() (*Config, error) {
	loader := loaders.NewChainLoader(
		loaders.NewEnvloader(),
		loaders.NewFileLoader(".env"),
		loaders.NewFlagLoader(),
	)

	// creating default config variables where possible
	serverCfg := Server{
		Environment: "prod",
	}
	apiCfg := API{
		Port: ":8080",
	}
	gslbCfg := GSLB{}

	configs := []any{
		&serverCfg,
		&apiCfg,
		&gslbCfg,
	}

	for _, cfg := range configs {
		err := loader.Load(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Config{
		Server: serverCfg,
		API:    apiCfg,
		GSLB:   gslbCfg,
	}, nil
}
