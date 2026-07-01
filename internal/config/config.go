package config

import (
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

var cfg *Config

type Config struct {
	Server        server   `mapstructure:"server"`
	Api           api      `mapstructure:"api"`
	SplitDNS      splitDns `mapstructure:"split_dns"`
	Gslb          gslb     `mapstructure:"gslb"`
	Jwt           jwt
	Webhooks      webhooks `mapstructure:"webhooks"`
	Valkey        valkey   `mapstructure:"valkey"`
	secretsLoaded int
	secretsTotal  int
}

func Server() *server {
	return &cfg.Server
}

func API() *api {
	return &cfg.Api
}

func SplitDNS() *splitDns {
	return &cfg.SplitDNS
}

func GSLB() *gslb {
	return &cfg.Gslb
}

func JWT() *jwt {
	return &cfg.Jwt
}

func Webhooks() *webhooks {
	return &cfg.Webhooks
}

func Valkey() *valkey {
	return &cfg.Valkey
}

func init() {
	var err error
	loadTimeStart := time.Now()

	cfg, err = new()
	if err != nil {
		log.Fatalf("unable to load config: %s", err.Error())
	}

	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level:       cfg.Server.LogLevel(),
		ReplaceAttr: bslog.BaseReplaceAttr,
	}

	switch cfg.Server.Env {
	case "dev", "development", "DEV", "DEVELOPMENT":
		handler = bslog.NewHandler(
			os.Stdout, // log output
			// slog handler factory
			func(w io.Writer) slog.Handler {
				return slog.NewTextHandler(w, handlerOpts)
			},
			// options
			bslog.InDevMode(),
			bslog.WithColor(),
		)

	case "prod", "production", "PROD", "PRODUCTION":
		handler = bslog.NewHandler(
			os.Stdout,
			func(w io.Writer) slog.Handler {
				return slog.NewJSONHandler(w, handlerOpts)
			},
			bslog.WithSplunkMultiHandler("<secret>", "<splunk_index>", slog.LevelInfo),
		)
	default:
		handler = bslog.NewHandler(
			os.Stdout, // log output
			// slog handler factory
			func(w io.Writer) slog.Handler {
				return slog.NewTextHandler(w, handlerOpts)
			},
			// options
			bslog.InDevMode(),
			bslog.WithColor(),
		)
	}

	slog.SetDefault(slog.New(handler))
	bslog.Info("config-loaded", slog.Any("config", cfg), slog.Int64("duration_ms", time.Since(loadTimeStart).Milliseconds()))
}

func (c *Config) LogValue() slog.Value {
	return slog.StringValue("un-implemented LogValue")
}

// server configuration
type server struct {
	Env         string `mapstructure:"env"`
	LUA_SANDBOX string `mapstructure:"lua_sandbox"`
	LOG_LEVEL   string `mapstructure:"log_level"`
}

func (s *server) Environment() string {
	return s.Env
}

func (s *server) LuaSandbox() string {
	return s.LUA_SANDBOX
}

func (s *server) LogLevel() slog.Level {
	switch s.LOG_LEVEL {
	case "healthcheck", "HEALTHCHECK":
		return bslog.LevelHealthCheck
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	case "fatal", "FATAL":
		return bslog.LevelFatal
	default:
		return slog.LevelDebug
	}
}

// API configuration
type api struct {
	PORT string `mapstructure:"port"`
}

func (a *api) Port() string {
	return a.PORT
}

type splitDns struct {
	Enabled bool     `mapstructure:"enabled"`
	Views   []string `mapstructure:"views"`
}

func (s *splitDns) Enable() bool {
	return s.Enabled
}

func (s *splitDns) DNSViews() []string {
	return s.Views
}

// gslb configuration
type gslb struct {
	ZONE         string
	NAMESERVER   string
	PollInterval string `mapstructure:"poll_interval"`
	SERVERS      string `mapstructure:"dnsdist_servers_file"`
}

func (g *gslb) Zone() string {
	return g.ZONE
}

func (g *gslb) Nameserver() string {
	return g.NAMESERVER
}

func (g *gslb) Poll() (timesutil.Duration, error) {
	duration, err := timesutil.FromString(g.PollInterval)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func (g *gslb) Servers() string {
	return g.SERVERS
}

type jwt struct {
	SECRET string
	USER   string
}

func (jwt *jwt) Secret() []byte {
	return []byte(jwt.SECRET)
}

func (jwt *jwt) User() string {
	return jwt.USER
}

type webhooks struct {
	Enabled bool
	Notices notifications `mapstructure:"notifications"`
	Mq      mq            `mapstructure:"mq"`
}

func (w *webhooks) Enable() bool {
	return w.Enabled
}

func (w *webhooks) Notifications() *notifications {
	return &w.Notices
}

func (w *webhooks) MQ() *mq {
	return &w.Mq
}

type notifications struct {
	SlackNotifier slack `mapstructure:"slack"`
}

func (n *notifications) Slack() *slack {
	return &n.SlackNotifier
}

type slack struct {
	Enabled        bool
	APP_TOKEN      string
	BOT_TOKEN      string
	SIGNING_SECRET string
}

func (s *slack) Enable() bool {
	return s.Enabled
}

func (s *slack) AppToken() string {
	return s.APP_TOKEN
}

func (s *slack) BotToken() string {
	return s.BOT_TOKEN
}

func (s *slack) SigningSecret() string {
	return s.SIGNING_SECRET
}

type mq struct {
	Usr      string
	Passwd   string
	EndPoint string
	PORT     string
}

func (mq *mq) User() string {
	return mq.Usr
}

func (mq *mq) Pass() string {
	return mq.Passwd
}

func (mq *mq) Endpoint() string {
	return mq.EndPoint
}

func (mq *mq) Port() string {
	return mq.PORT
}

type valkey struct {
	Enabled bool
	Addr    string
	USER    string
	PASS    string
}

func (v *valkey) Address() string {
	return v.Addr
}

func (v *valkey) User() string {
	return v.USER
}

func (v *valkey) Password() string {
	return v.PASS
}

/*
func newConfig() (*Config, error) {
	fileLoader, err := loaders.NewFileLoader(
		".env",
		"./secrets",
	)

	if err != nil {
		return nil, err
	}

	loader := loaders.NewChainLoader(
		loaders.NewEnvloader(),
		fileLoader,
		loaders.NewFlagLoader(),
	)

	// creating default config variables where possible
	serverCfg := server{
		ENV: "prod",
	}

	apiCfg := API{
		PORT: ":8080",
	}

	gslbCfg := gslb{
		POLLINTERVAL: "1m",
	}

	jwtCfg := jwt{}
	slackCfg := slack{}
	mqCfg := mq{}

	valkeyCfg := valkey{Addr: "localhost:6379"}

	configs := []any{
		&serverCfg,
		&apiCfg,
		&gslbCfg,
		&jwtCfg,
		&slackCfg,
		&mqCfg,
		&valkeyCfg,
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
		slack:  slackCfg,
		mq:     mqCfg,
		valkey: valkeyCfg,
	}, nil
}
*/
