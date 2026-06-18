package spoofs

type Spoof struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	FQDN string `json:"fqdn"`
	IP   string `json:"ip"`
}
