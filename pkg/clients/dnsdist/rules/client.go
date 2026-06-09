package rules

import (
	"fmt"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport"
)

type Client interface {
	// adds a new rule+action pair
	Add(name string, rule Rule, action Action) error

	// removes a rule
	Remove(id string) error

	// lists all rules
	List() (string, error)

	// checks if a rule with the given name exists
	Exist(name string) (bool, error)

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
	cmd := fmt.Sprintf("addAction(%s, %s, {name='%s'})", rule.luaRule(), action.luaAction(), name)
	response, err := c.transport.Execute(cmd)
	if strings.Contains(response, "Error") {
		return fmt.Errorf("failed to add rule: dnsdist returned lua error: %s: for command: %s", response, cmd)
	}
	return err
}

// removes a rule
func (c *rulesClient) Remove(id string) error {
	cmd := fmt.Sprintf("rmRule('%s')", id)
	response, err := c.transport.Execute(cmd)
	if strings.Contains(response, "Error") {
		return fmt.Errorf("failed to remove rule: dnsdist returned lua error: %s: for command: %s", response, cmd)
	}
	return err
}

// lists all rules
func (c *rulesClient) List() (string, error) {
	return c.transport.Execute("showRules()")
}

// checks wether a rule with the given name exist
func (c *rulesClient) Exist(name string) (bool, error) {
	cmd := fmt.Sprintf("tostring(getRule('%s') ~= nil)", name)
	
	response, err := c.transport.Execute(cmd)
	if err != nil {
		return false, err
	}

	if strings.Contains(response, "Error") {
		return false, fmt.Errorf("failed to check rule existence: dnsdist returned lua error: %s: for command: %s", response, cmd)
	}

	return response == "true", nil
}

// removes all rules
func (c *rulesClient) Clear() error {
	_, err := c.transport.Execute("clearRules()")
	return err
}
