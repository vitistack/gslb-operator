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
		log.Fatalf("unable to load config: %s", err)
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
	env        string `env:"SRV_ENV" flag:"env"`
	datacenter string `env:"SRV_DATACENTER" flag:"datacenter"`
}

func (s *Server) Env() string {
	return s.env
}

func (s *Server) Datacenter() string {
	return s.datacenter
}

// API configuration
type API struct {
	port string `env:"API_PORT" flag:"port"`
}

func (a *API) Port() string {
	return a.port
}

// GSLB configuration
type GSLB struct {
	zone       string `env:"GSLB_ZONE" flag:"gslb-zone"`
	nameServer string `env:"GSLB_NAMESERVER" flag:"gslb-nameserver"`
}

func (g *GSLB) Zone() string {
	return g.zone
}

func (g *GSLB) NameServer() string {
	return g.nameServer
}

func newConfig() (*Config, error) {
	loader := loaders.NewChainLoader(
		loaders.NewEnvloader(),
		loaders.NewFileLoader(".env"),
		loaders.NewFlagLoader(),
	)

	// creating default config variables where possible
	serverCfg := Server{
		env: "prod",
	}
	apiCfg := API{
		port: ":8080",
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
