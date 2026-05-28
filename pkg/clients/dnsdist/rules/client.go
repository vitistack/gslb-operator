package rules

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport"
)

type Client interface {
	// adds a new rule+action pair
	Add(name string, rule Rule, action Action) error

	// removes a rule
	Remove(id string) error

	// lists all rules
	List() (string, error)

	// removes all rules
	Clear() error
}

type rulesClient struct {
	transport transport.Transport
}

func New(transport transport.Transport) Client {
	return &rulesClient{
		transport: transport,
	}
}

// adds a new rule+action pair
func (c *rulesClient) Add(name string, rule Rule, action Action) error {
	_, err := c.transport.Execute(fmt.Sprintf("addAction(%s, %s)", rule.luaRule(), action.luaAction()))
	return err
}

// removes a rule
func (c *rulesClient) Remove(id string) error {
	_, err := c.transport.Execute(fmt.Sprintf("rmRule(%q)", id))
	return err
}

// lists all rules
func (c *rulesClient) List() (string, error) {
	return c.transport.Execute("showRules()")
}

// removes all rules
func (c *rulesClient) Clear() error {
	_, err := c.transport.Execute("clearRules()")
	return err
}
