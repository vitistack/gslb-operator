package dnsdist

import (
	"log"
	"testing"
)

func TestNewClient(t *testing.T) {
	_, err := NewClient(
		"M2YQKiPEDzeWHUFjejVOd+QHmMVmm2SuYG7vSXdaIkE=",
		WithNumRetriesOnCommandFailure(0),
	)
	if err != nil {
		t.Errorf("could not create client: %v", err.Error())
	}

}

func TestCommand(t *testing.T) {
	client, err := NewClient(
		"M2YQKiPEDzeWHUFjejVOd+QHmMVmm2SuYG7vSXdaIkE=",
		WithNumRetriesOnCommandFailure(0),
	)
	if err != nil {
		t.Errorf("could not create client: %v", err.Error())
	}

	resp, err := client.command("showServers()")
	if err != nil {
		t.Errorf("error while sending command to server: %v", err.Error())
	}
	log.Printf("got response: \n%v", resp)

	client.Disconnect()
}

func TestAddDomainSpoof(t *testing.T) {
	client, err := NewClient(
		"M2YQKiPEDzeWHUFjejVOd+QHmMVmm2SuYG7vSXdaIkE=",
		WithNumRetriesOnCommandFailure(0),
	)
	if err != nil {
		t.Errorf("could not create client: %v", err.Error())
	}

	err = client.AddDomainSpoof("test.nhn.no", []string{"10.10.0.1", "10.10.0.2"})
	if err != nil {
		t.Errorf("failed to create DomainSpoof")
	}
}
