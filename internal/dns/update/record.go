package update

import "codeberg.org/miekg/dns"

type Record struct {
	dns.RR
	Datacenter string
	ID         string
}
