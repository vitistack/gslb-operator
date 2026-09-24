package model

type DNSDISTServer struct {
	Name string `json:"name" yaml:"name" mapstructure:"name"`
	Host string `json:"host" yaml:"host" mapstructure:"host"`
	Port uint16 `json:"port" yaml:"port" mapstructure:"port"`
	Key  string `json:"key" yaml:"key" mapstructure:"key"`
	View string `json:"view" yaml:"view" mapstructure:"view"`
}
