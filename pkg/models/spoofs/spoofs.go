package spoofs

type Spoof struct {
	FQDN string `json:"fqdn"`
	IP   string `json:"ip"`
	DC   string `json:"datacenter"` // when active override, DC == "OVERRIDE"
}

func (s Spoof) Key() string {
	return s.FQDN + ":" + s.DC
}
