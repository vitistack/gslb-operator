package config

import (
	"log"

	"github.com/vitistack/gslb-operator/pkg/loaders"
)

var cfg *Config

func init() {
	var err error
	cfg, err = newConfig()
	if err != nil {
		log.Fatalf("unable to load config: %s", err.Error())
	}
}

type Config struct {
	server Server
	api    API
	gslb   GSLB
}

func GetInstance() *Config {
	return cfg
}

func (c *Config) Server() *Server {
	return &c.server
}

func (c *Config) API() *API {
	return &c.api
}

func (c *Config) GSLB() *GSLB {
	return &c.gslb
}

// Server configuration
type Server struct {
	ENV string `env:"SRV_ENV" flag:"env"`
	DC  string `env:"SRV_DATACENTER" flag:"datacenter"`
}

func (s *Server) Env() string {
	return s.ENV
}

func (s *Server) Datacenter() string {
	return s.DC
}

// API configuration
type API struct {
	PORT string `env:"API_PORT" flag:"port"`
}

func (a *API) Port() string {
	return a.PORT
}

// GSLB configuration
type GSLB struct {
	ZONE       string `env:"GSLB_ZONE" flag:"gslb-zone"`
	NAMESERVER string `env:"GSLB_NAMESERVER" flag:"gslb-nameserver"`
}

func (g *GSLB) Zone() string {
	return g.ZONE
}

func (g *GSLB) NameServer() string {
	return g.NAMESERVER
}

func newConfig() (*Config, error) {
	loader := loaders.NewChainLoader(
		loaders.NewEnvloader(),
		loaders.NewFileLoader(".env"),
		loaders.NewFlagLoader(),
	)

	// creating default config variables where possible
	serverCfg := Server{
		ENV: "prod",
	}
	apiCfg := API{
		PORT: ":8080",
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
		server: serverCfg,
		api:    apiCfg,
		gslb:   gslbCfg,
	}, nil
}
