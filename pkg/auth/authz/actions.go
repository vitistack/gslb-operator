package authz

import (
	"net/http"
	"strings"
)

const (
	ActionRead   = "read"
	ActionWrite  = "write"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

var knownActions = []string{ActionRead, ActionWrite, ActionUpdate, ActionDelete}

// ActionFor maps an HTTP method to its action; ok is false for unmapped methods.
func ActionFor(method string) (string, bool) {
	switch method {
	case http.MethodGet, http.MethodHead:
		return ActionRead, true
	case http.MethodPost:
		return ActionWrite, true
	case http.MethodPut, http.MethodPatch:
		return ActionUpdate, true
	case http.MethodDelete:
		return ActionDelete, true
	}
	return "", false
}

// ObjectType drops wildcard segments: "/services/{memberOf}/members/{id}" -> "services/members".
func ObjectType(pattern string) string {
	var segs []string
	for s := range strings.SplitSeq(pattern, "/") {
		if s == "" || strings.HasPrefix(s, "{") {
			continue
		}
		segs = append(segs, s)
	}
	return strings.Join(segs, "/")
}
