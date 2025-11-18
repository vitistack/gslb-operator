package dnsdist

import (
	"log"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient("M2YQKiPEDzeWHUFjejVOd+QHmMVmm2SuYG7vSXdaIkE=", "127.0.0.1", "5199", time.Second*5)
	if err != nil {
		t.Errorf("could not create client: %v", err.Error())
	}
	client.Disconnect()
}

func TestCommand(t *testing.T) {
	client, err := NewClient("M2YQKiPEDzeWHUFjejVOd+QHmMVmm2SuYG7vSXdaIkE=", "127.0.0.1", "5199", time.Second*5)
	if err != nil {
		t.Errorf("could not create client: %v", err.Error())
	}

	resp, err := client.Command("showServers()")
	if err != nil {
		t.Errorf("error while sending command to server: %v", err.Error())
	}
	log.Printf("got response: \n%v", resp)

	client.Disconnect()
}
