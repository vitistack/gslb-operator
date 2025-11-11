package main

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
)

func main() {
	fetcher := dns.NewZoneFetcherWithAutoPoll("gslb.test.dns.nhn.no.", "nsh1.nhn.no:53", dns.DEFAULT_POLL_INTERVAL)
	mgr := manager.NewManager(10, 10)

	handler := dns.NewHandler(fetcher, mgr, &dns.Updater{})
	handler.Start()
	time.Sleep(time.Second * 11)
	handler.Stop()
}
