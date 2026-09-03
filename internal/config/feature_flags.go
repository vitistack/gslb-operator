package config

type FeatureFlags struct {
	WebHooks WebhooksFeatureFlag    `mapstructure:"webhooks"`
	MQ       struct{ Enabled bool } `mapstructure:"mq"`
	Valkey   struct{ Enabled bool } `mapstructure:"valkey"`
	SplitDNS struct{ Enabled bool } `mapstructure:"split_dns"`
}

type WebhooksFeatureFlag struct {
	Enabled       bool
	Notifications struct {
		Slack struct{ Enabled bool } `mapstructure:"slack"`
	} `mapstructure:"notifications"`
}
