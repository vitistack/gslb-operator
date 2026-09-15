package authc

import (
	"encoding/json"
	"io"
	"net/http"
)

// Authenticator verifies the identity behind a login using one method.
// Add a new login method by implementing this and passing it to NewService.
type Authenticator interface {
	Method() string                                  // unique id, e.g. "bootstrap-key"
	Class() AuthClass                                // M2M or C2M
	Detect(r *http.Request) bool                     // can it handle this request?
	Authenticate(r *http.Request) (Principal, error) // verify credential → identity
}

type Dispatcher struct {
	ordered  []Authenticator
	byMethod map[string]Authenticator
}

func NewDispatcher(as ...Authenticator) *Dispatcher {
	d := &Dispatcher{byMethod: make(map[string]Authenticator, len(as))}
	for _, a := range as {
		d.ordered = append(d.ordered, a)
		d.byMethod[a.Method()] = a
	}
	return d
}

// Resolve selects an authenticator dynamically: an explicit X-Auth-Method
// header wins; otherwise the first authenticator that detects its credential.
func (d *Dispatcher) Resolve(r *http.Request) (Authenticator, error) {
	if m := r.Header.Get("X-Auth-Method"); m != "" {
		a, ok := d.byMethod[m]
		if !ok {
			return nil, ErrUnknownMethod
		}
		return a, nil
	}

	for _, a := range d.ordered {
		if a.Detect(r) {
			return a, nil
		}
	}
	return nil, ErrNoMethodDetected
}

func (d *Dispatcher) Authenticate(r *http.Request) (Principal, error) {
	var payload LoginPayload
	if r.Body != nil {
		// tolerate empty body: cookie-based (C2M) methods carry no JSON payload
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload)
	}
	r = r.WithContext(WithLoginPayload(r.Context(), payload))

	authr, err := d.Resolve(r)
	if err != nil {
		return Principal{}, err
	}
	return authr.Authenticate(r)
}
