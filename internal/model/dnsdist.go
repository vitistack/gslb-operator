package model

import "net"

type DNSDISTServer struct {
	Name string `json:"name"`
	Host net.IP `json:"host"`
	Port uint16 `json:"port"`
	Key  string `json:"key"`
	View string `json:"view"`
}
