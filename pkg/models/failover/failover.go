package failover

// Failover a service to another DC
type Failover struct {
	NextHealthy bool   `json:"nextHealthy"`
	Datacenter  string `json:"datacenter"`
}
