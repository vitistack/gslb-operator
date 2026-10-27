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

type AssignRoles struct {
	Roles []string `json:"roles"`
}

type AssignAttributes struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}
