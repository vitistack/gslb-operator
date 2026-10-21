package auth

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type LoginPayload struct {
	ClientID        string `json:"clientId"`
	PublicKey       string `json:"pubKey"`          // base64 Ed25519 (bootstrap enrolment)
	ClientAssertion string `json:"clientAssertion"` // signed JWT (private-key-jwt)
}

type AssignRoles struct {
	Roles []string `json:"roles"`
}

type AssignAttributes struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}
