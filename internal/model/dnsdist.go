package model

import "net"

type DNSDISTServer struct {
	Name string `json:"name"`
	Host net.IP `json:"host"`
	Port string `json:"port"`
	Key  string `json:"key"`
}
