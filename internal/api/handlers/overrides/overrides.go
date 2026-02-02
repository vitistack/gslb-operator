// All the overrides are manual overrides created from "the outside"
// this is in case of "emergency" when manual overrides are necessary.
// this means that once an override is made. An update will be made to update dnsdist
package overrides_service

import "net/http"

type OverrideService struct {

}

func NewOverrideService() *OverrideService {
	return &OverrideService{}
}

func (os *OverrideService) GetOverrides(w http.ResponseWriter, r *http.Request) {

}

func (os *OverrideService) CreateOverride(w http.ResponseWriter, r *http.Request) {

}

func (os *OverrideService) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	
}
