// All the overrides are manual overrides created from "the outside"
// this is in case of "emergency" when manual overrides are necessary.
// this means that once an override is made. An update will be made to update dnsdist
package handler

import "net/http"

func (h *Handler) GetOverrides(w http.ResponseWriter, r *http.Request) {

}

func (h *Handler) CreateOverride(w http.ResponseWriter, r *http.Request) {

}

func (h *Handler) DeleteOverride(w http.ResponseWriter, r *http.Request) {

}
