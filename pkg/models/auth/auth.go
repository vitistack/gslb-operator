package auth

type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int    `json:"expiresIn"`
}

func (tr TokenResponse) String() string {
	return tr.TokenType + " " + tr.AccessToken
}

type LoginPayload struct {
	ClientID        string `json:"clientId"`
	PublicKey       string `json:"publicKey"`       // base64 Ed25519 (bootstrap enrolment)
	ClientAssertion string `json:"clientAssertion"` // signed JWT (private-key-jwt)
}

type Grant struct {
	Roles      *[]string            `json:"roles,omitempty"`
	Attributes *map[string][]string `json:"attributes,omitempty"`
}

type Enrollment struct {
	ID         string              `json:"id"`
	Roles      []string            `json:"roles"`
	Attributes map[string][]string `json:"attrs,omitempty"`
}
