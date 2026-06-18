package update

import "codeberg.org/miekg/dns"

type Record struct {
	dns.RR
	UUID string
	View string
}
