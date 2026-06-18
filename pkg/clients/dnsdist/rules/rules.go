package rules

import (
	"fmt"
	"strings"
)

type GlobalRuleOptions struct {
	Name *string
	UUID *string
}

func (t *GlobalRuleOptions) luaTable() string {
	if t == nil {
		return ""
	}

	var parts []string
	if t.Name != nil {
		parts = append(parts, fmt.Sprintf("name='%s'", *t.Name))
	}

	if t.UUID != nil {
		parts = append(parts, fmt.Sprintf("uuid='%s'", *t.UUID))
	}

	return "{" + strings.Join(parts, ",") + "}"
}

type ListOptions struct {
	ShowUUIDs         *bool
	TruncateRuleWidth *int
}

func (t *ListOptions) luaTable() string {
	if t == nil {
		return ""
	}

	var parts []string
	if t.ShowUUIDs != nil {
		parts = append(parts, fmt.Sprintf("showUUIDs=%v", *t.ShowUUIDs))
	}

	if t.TruncateRuleWidth != nil {
		parts = append(parts, fmt.Sprintf("truncateRuleWidth=%d", *t.TruncateRuleWidth))
	}

	return "{" + strings.Join(parts, ",") + "}"
}

type allRule struct{}

func (allRule) luaRule() string { return "AllRule()" }

// AllRule matches every incoming query.
func AllRule() Rule { return allRule{} }

type qnameRule struct{ name string }

// QNameRule matches queries for the exact DNS name.
func (r qnameRule) luaRule() string { return fmt.Sprintf("QNameRule('%s')", r.name) }
func QNameRule(name string) Rule    { return qnameRule{name} }

type qnameSuffixRule struct{ names []string }

// QNameSuffixRule matches queries whose qname ends with any of the given suffixes.
func QNameSuffixRule(names ...string) Rule { return qnameSuffixRule{names} }
func (r qnameSuffixRule) luaRule() string {
	quoted := make([]string, len(r.names))
	for i, n := range r.names {
		quoted[i] = fmt.Sprintf("'%s'", n)
	}
	return fmt.Sprintf("QNameSuffixRule({%s})", strings.Join(quoted, ", "))
}

type qtypeRule struct{ qt QType }

// QTypeRule matches queries of the given DNS type.
func QTypeRule(qt QType) Rule       { return qtypeRule{qt} }
func (r qtypeRule) luaRule() string { return fmt.Sprintf("QTypeRule(%d)", r.qt) }

type netmaskGroupRule struct{ netmasks []string }

func NetmaskGroupRule(netmasks ...string) Rule { return netmaskGroupRule{netmasks} }
func (r netmaskGroupRule) luaRule() string {
	quoted := make([]string, len(r.netmasks))
	for i, n := range r.netmasks {
		quoted[i] = fmt.Sprintf("'%s'", n)
	}
	return fmt.Sprintf("NetmaskGroupRule(newNMG({%s}))", strings.Join(quoted, ", "))
}

type andRule struct{ rules []Rule }

// AndRule matches only when all of the given rules match.
func AndRule(rules ...Rule) Rule { return andRule{rules} }
func (r andRule) luaRule() string {
	parts := make([]string, len(r.rules))
	for i, sub := range r.rules {
		parts[i] = sub.luaRule()
	}
	return fmt.Sprintf("AndRule({%s})", strings.Join(parts, ", "))
}

type orRule struct{ rules []Rule }

// OrRule matches when any of the given rules match.
func OrRule(rules ...Rule) Rule { return orRule{rules} }
func (r orRule) luaRule() string {
	parts := make([]string, len(r.rules))
	for i, sub := range r.rules {
		parts[i] = sub.luaRule()
	}
	return fmt.Sprintf("OrRule({%s})", strings.Join(parts, ", "))
}

type notRule struct{ rule Rule }

// NotRule inverts the given rule.
func NotRule(rule Rule) Rule      { return notRule{rule} }
func (r notRule) luaRule() string { return fmt.Sprintf("NotRule(%s)", r.rule.luaRule()) }
