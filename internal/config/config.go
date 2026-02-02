package config

import (
	"log"
	"log/slog"
	"os"

	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/loaders"
)

var cfg *Config

func init() {
	var err error
	cfg, err = newConfig()
	if err != nil {
		log.Fatalf("unable to load config: %s", err.Error())
	}

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: bslog.BaseReplaceAttr,
	})

	switch cfg.server.ENV {
	case "dev", "development", "DEV", "DEVELOPMENT":
		handler = bslog.NewHandler(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level:       slog.LevelDebug,
				ReplaceAttr: bslog.BaseReplaceAttr,
			}),
			bslog.InDevMode(),
		)
	case "prod", "production", "PROD", "PRODUCTION":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: bslog.BaseReplaceAttr,
		})
	}

	slog.SetDefault(slog.New(handler))
}

type Config struct {
	server Server
	api    API
	gslb   GSLB
	jwt    JWT
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

func (c *Config) JWT() *JWT {
	return &c.jwt
}

// Server configuration
type Server struct {
	ENV         string `env:"SRV_ENV" flag:"env"`
	DC          string `env:"SRV_DATACENTER" flag:"datacenter"`
	LUA_SANDBOX string `env:"SRV_LUA_SANDBOX" flag:"lua-sandbox"`
}

func (s *Server) Env() string {
	return s.ENV
}

func (s *Server) Datacenter() string {
	return s.DC
}

func (s *Server) LuaSandbox() string {
	return s.LUA_SANDBOX
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
	ZONE         string `env:"GSLB_ZONE" flag:"gslb-zone"`
	NAMESERVER   string `env:"GSLB_NAMESERVER" flag:"gslb-nameserver"`
	POLLINTERVAL string `env:"GSLB_POLL_INTERVAL" flag:"poll-interval"`
	UPDATERHOST  string `env:"GSLB_UPDATER_HOST" flag:"updater-host"`
}

func (g *GSLB) Zone() string {
	return g.ZONE
}

func (g *GSLB) NameServer() string {
	return g.NAMESERVER
}

func (g *GSLB) PollInterval() (timesutil.Duration, error) {
	duration, err := timesutil.FromString(g.POLLINTERVAL)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func (g *GSLB) UpdaterHost() string {
	return g.UPDATERHOST
}

type JWT struct {
	SECRET string `env:"JWT_SECRET"`
	USER   string `env:"JWT_USER"`
}

func (jwt *JWT) Secret() []byte {
	return []byte(jwt.SECRET)
}

func (jwt *JWT) User() string {
	return jwt.USER
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
	jwtCfg := JWT{}

	configs := []any{
		&serverCfg,
		&apiCfg,
		&gslbCfg,
		&jwtCfg,
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
		jwt:    jwtCfg,
	}, nil
}
