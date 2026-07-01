package config

type FeatureFlags struct {
	WebHooks           WebhooksFeatureFlag    `mapstructure:"webhooks"`
	Valkey             struct{ Enabled bool } `mapstructure:"valkey"`
	SplitDNS           struct{ Enabled bool } `mapstructure:"split-dns"`
}

type WebhooksFeatureFlag struct {
	Enabled       bool
	Notifications struct {
		Slack struct{ Enabled bool } `mapstructure:"slack"`
	} `mapstructure:"notifications"`
}
