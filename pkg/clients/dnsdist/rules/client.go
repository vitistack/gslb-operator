package rules

import (
	"fmt"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport"
)

type Client interface {
	// adds a new rule+action pair
	Add(rule Rule, action Action, opts ...GlobalRuleOptions) error

	// calls remove and add in a single line
	// TODO: Should i even have this?
	// Update(name string, rule Rule, action Action) error

	// removes a rule
	Remove(id string) error

	// lists all rules
	List(opts ...Table) (string, error)

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
func (c *rulesClient) Add(rule Rule, action Action, opts ...GlobalRuleOptions) error {
	cmd := fmt.Sprintf("addAction(%s, %s)", rule.luaRule(), action.luaAction())

	if len(opts) > 0 {
		opt := &opts[0]
		table := opt.luaTable()
		if table != "" {
			cmd = fmt.Sprintf("addAction(%s, %s, %s)", rule.luaRule(), action.luaAction(), table)
		}
	}

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
func (c *rulesClient) List(opts ...Table) (string, error) {
	cmd := "showRules()"

	if len(opts) > 0 {
		opt := opts[0]
		table := opt.luaTable()
		if table != "" {
			cmd = fmt.Sprintf("showRules(%s)", table)
		}
	}

	return c.transport.Execute(cmd)
}

// checks wether a rule with the given name exist
func (c *rulesClient) Exist(id string) (bool, error) {
	cmd := fmt.Sprintf("tostring(getRule('%s') ~= nil)", id)

	response, err := c.transport.Execute(cmd)
	if err != nil {
		return false, err
	}

	if strings.Contains(response, "Error") {
		return false, fmt.Errorf("failed to check for rule: dnsdist returned lua error: %s: for command: %s", response, cmd)
	}

	return strings.Contains(response, "true"), nil
}

// removes all rules
func (c *rulesClient) Clear() error {
	_, err := c.transport.Execute("clearRules()")
	return err
}
