package servers

type Client interface {
	// returns the raw showServers() output.
	List() (string, error)
	// adds a new backend. addr is "ip:port".
	Add(addr string) error
	// removes a backend by index, UUID or name.
	Remove(id string) error
	// administratively marks a backend as up.
	SetUp(id string) error
	// administratively marks a backend as down.
	SetDown(id string) error
}
