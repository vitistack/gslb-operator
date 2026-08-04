package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var secretsKeyMap = map[string]string{
	"GSLB_NAMESERVER": "gslb.nameserver",
	"GSLB_ZONE":       "gslb.zone",
	"JWT_SECRET":      "jwt.secret",
	"JWT_USER":        "jwt.user",
}

func new() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/app")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config file: %w", err)
		}
		return nil, fmt.Errorf("un-expected error while loading configuration: %w", err)
	}

	var featureFlags FeatureFlags
	if err := v.Unmarshal(&featureFlags); err != nil {
		return nil, fmt.Errorf("unmarshalling feature flags: %w", err)
	}

	loaded, total, err := loadSecrets(v, featureFlags, "./secrets")
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	cfg.secretsLoaded = loaded
	cfg.secretsTotal = total
	v = nil

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.env", "prod")
	v.SetDefault("server.lua_sandbox", "sandbox.lua")
	v.SetDefault("server.log_level", "HEALTHCHECK")

	v.SetDefault("api.port", ":8080")

	v.SetDefault("split_dns.enabled", true)
	v.SetDefault("split_dns.views", []string{"default"})
	v.SetDefault("split_dns.defaultView", "default")

	v.SetDefault("gslb.poll_interval", "1m")
	v.SetDefault("gslb.dnsdist_servers_file", "./secrets/GSLB_DNSDIST_SERVERS")

	v.SetDefault("webhooks.enabled", false)
	v.SetDefault("webhooks.mq.endpoint", "localhost")
	v.SetDefault("webhooks.mq.port", "5672")

	v.SetDefault("valkey.enabled", true)
	v.SetDefault("valkey.addr", "localhost:6379")
}

func loadSecrets(v *viper.Viper, flags FeatureFlags, dir string) (loaded, total int, err error) {
	if flags.Valkey.Enabled {
		secretsKeyMap["VK_PASS"] = "valkey.pass"
		secretsKeyMap["VK_USER"] = "valkey.user"
	}

	if flags.WebHooks.Enabled {
		secretsKeyMap["MQ_PASS"] = "webhooks.mq.pass"
		secretsKeyMap["MQ_USER"] = "webhooks.mq.user"
		if flags.WebHooks.Notifications.Slack.Enabled {
			secretsKeyMap["SLACK_APP_TOKEN"] = "webhooks.notifications.slack.app_token"
			secretsKeyMap["SLACK_BOT_TOKEN"] = "webhooks.notifications.slack.bot_token"
			secretsKeyMap["SLACK_SIGNING_SECRET"] = "webhooks.notifications.slack.signing_secret"
		}
	}

	total = len(secretsKeyMap)

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return loaded, total, nil
	}

	if err != nil {
		return loaded, total, fmt.Errorf("failed to load secrets directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		key, ok := secretsKeyMap[entry.Name()]
		if !ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return loaded, total, fmt.Errorf("reading secret %s: %w", entry.Name(), err)
		}
		value := strings.TrimSpace(string(raw))
		if value != "" {
			v.Set(key, value)
			loaded++
		}
	}

	return loaded, total, nil
}
