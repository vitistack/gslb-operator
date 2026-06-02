package pools

type Client interface {
	// returns the raw showPools() output.
	List() (string, error)
}
