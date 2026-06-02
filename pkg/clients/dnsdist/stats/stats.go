package stats

type Client interface {
	// returns the dnsdist version string.
	Version() (string, error)
	// returns the full dumpStats() output.
	Dump() (string, error)
	// returns the raw getStatisticsCounters() output (Lua table as string).
	Counters() (string, error)
}
