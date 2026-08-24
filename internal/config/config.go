package config

import (
	"fmt"
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
	Dns           splitDns `mapstructure:"split_dns"`
	Gslb          gslb     `mapstructure:"gslb"`
	Jwt           jwt      `mapstructure:"jwt"`
	Webhooks      webhooks `mapstructure:"webhooks"`
	Mq            mq       `mapstructure:"mq"`
	Valkey        valkey   `mapstructure:"valkey"`
	secretsLoaded int
	secretsTotal  int
}

func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Server.Env),
		slog.String("log_level", c.Server.LOG_LEVEL),
		slog.String("api_port", c.Api.PORT),

		slog.Bool("dns_enabled", c.Dns.Enabled),
		slog.Any("dns_views", c.Dns.Views),
		slog.String("dns_default_view", c.Dns.Default),

		slog.String("gslb_zone", c.Gslb.ZONE),
		slog.String("gslb_nameserver", c.Gslb.NS),
		slog.String("gslb_poll_interval", c.Gslb.PollInterval),

		slog.Bool("mq_enabled", c.Mq.Enable),
		slog.String("mq_url", fmt.Sprintf("%s:%s", c.Mq.EndPoint, c.Mq.PORT)),

		slog.Bool("webhooks_enabled", c.Webhooks.Enable),
		slog.Bool("slack_enabled", c.Webhooks.Notices.SlackNotifier.Enable),

		slog.Bool("valkey_enabled", c.Valkey.Enabled),
		slog.String("valkey_addr", c.Valkey.Addr),

		slog.String("secrets_loaded", fmt.Sprintf("%d/%d", c.secretsLoaded, c.secretsTotal)),
	)
}

func Server() *server {
	return &cfg.Server
}

func API() *api {
	return &cfg.Api
}

func DNS() *splitDns {
	return &cfg.Dns
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

func MQ() *mq {
	return &cfg.Mq
}

func Valkey() *valkey {
	return &cfg.Valkey
}

func init() {
	var err error
	loadTimeStart := time.Now()

	cfg, err = new()
	if err != nil {
		log.Printf("unable to load config, falling back to defaults: %s", err.Error())
		cfg = defaultConfig()
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
	Default string   `mapstructure:"defaultView"`
	Views   []string `mapstructure:"views"`
}

func (s *splitDns) Enable() bool {
	return s.Enabled
}

func (s *splitDns) DefaultView() string {
	return s.Default
}

func (s *splitDns) DNSViews() []string {
	return s.Views
}

// gslb configuration
type gslb struct {
	Status struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"status"`
	SITE         string `mapstructure:"site"`
	ZONE         string `mapstructure:"zone"`
	NS           string `mapstructure:"nameserver"`
	PollInterval string `mapstructure:"poll_interval"`
	SERVERS      string `mapstructure:"dnsdist_servers_file"`
}

func (g *gslb) Site() string {
	return g.SITE
}

func (g *gslb) StatusEnabled() bool {
	return g.Status.Enabled
}

func (g *gslb) Zone() string {
	return g.ZONE
}

func (g *gslb) Nameserver() string {
	return g.NS
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
	SECRET string `mapstructure:"secret"`
	USER   string `mapstructure:"user"`
}

func (jwt *jwt) Secret() []byte {
	return []byte(jwt.SECRET)
}

func (jwt *jwt) User() string {
	return jwt.USER
}

type webhooks struct {
	Enable  bool          `mapstructure:"enabled"`
	Notices notifications `mapstructure:"notifications"`
}

func (w *webhooks) Enabled() bool {
	return w.Enable
}

func (w *webhooks) Notifications() *notifications {
	return &w.Notices
}

type notifications struct {
	SlackNotifier slack `mapstructure:"slack"`
}

func (n *notifications) Slack() *slack {
	return &n.SlackNotifier
}

type slack struct {
	Enable         bool   `mapstructure:"enabled"`
	APP_TOKEN      string `mapstructure:"app_token"`
	BOT_TOKEN      string `mapstructure:"bot_token"`
	SIGNING_SECRET string `mapstructure:"signing_secret"`
}

func (s *slack) Enabled() bool {
	return s.Enable
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
	Enable   bool   `mapstructure:"enabled"`
	Usr      string `mapstructure:"user"`
	Passwd   string `mapstructure:"pass"`
	EndPoint string `mapstructure:"endpoint"`
	PORT     string `mapstructure:"port"`
}

func (mq *mq) Enabled() bool {
	return mq.Enable
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
	Addr    string `mapstructure:"addr"`
	USER    string `mapstructure:"user"`
	PASS    string `mapstructure:"pass"`
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
