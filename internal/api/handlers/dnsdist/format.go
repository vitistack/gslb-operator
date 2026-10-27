package dnsdist

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type outputFormat string

const (
	outputFormatJSON outputFormat = "json"
	outputFormatLua  outputFormat = "lua"
)

type outputEncoder interface {
	encode(http.ResponseWriter, []spoofs.Spoof) error
}

func newOutputEncoder(format string) (outputEncoder, error) {
	switch outputFormat(format) {
	case "", outputFormatJSON:
		return jsonEncoder{}, nil
	case outputFormatLua:
		return luaEncoder{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}

type jsonEncoder struct{}

func (jsonEncoder) encode(w http.ResponseWriter, result []spoofs.Spoof) error {
	return response.JSON(w, http.StatusOK, result)
}

type luaEncoder struct{}

func (luaEncoder) encode(w http.ResponseWriter, result []spoofs.Spoof) error {
	var builder strings.Builder

	for _, spoof := range result {
		builder.WriteString(rules.RenderAddAction(
			rules.QNameRule(spoof.FQDN),
			rules.SpoofAction(spoof.Address.Strings(), rules.SpoofActionOptions{TTL: new(30), AA: new(true)}),
			rules.GlobalRuleOptions{Name: &spoof.Name, UUID: &spoof.UUID},
		))
		builder.WriteByte('\n')
	}

	return response.Text(w, http.StatusOK, builder.String())
}
