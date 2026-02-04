package jwt

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/vitistack/gslb-operator/internal/api/routes"
)

type UserClaims struct {
	Name           string   `json:"name"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedRoutes  []string `json:"allowed_routes"`
	jwt.RegisteredClaims
}

type Role string

const (
	ADMIN Role = "ADMIN"
	RW    Role = "RW" // Read/Write
	RO    Role = "RO" // Read-Only
)

func getUserClaims(name string) (UserClaims, bool) {
	for _, claimsForRole := range endpointUserClaims {
		for _, userClaims := range claimsForRole {
			if userClaims.Name == name {
				return userClaims, true
			}
		}
	}
	return UserClaims{}, false
}

func Validate(ctx context.Context, tokenString string) (*JWTError, error) {
	tokenString = strings.Trim(tokenString, " ")
	token, err := jwt.ParseWithClaims(
		tokenString,
		&UserClaims{},
		func(t *jwt.Token) (any, error) {
			secret := GetInstance().issuer.secret
			return secret, nil
		},
		jwt.WithJSONNumber(),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{
			GetInstance().GetSigningMethod().Alg(),
		}),
	)
	if err != nil {
		return Errors[ErrUnAuthorized], fmt.Errorf("invalid token: %w", err)
	}

	requestClaims, ok := token.Claims.(*UserClaims)
	if !ok {
		return Errors[ErrUnAuthorized], fmt.Errorf("invalid claims: unable to locate user claims section")
	}

	userClaims, ok := getUserClaims(requestClaims.Name)
	if !ok {
		return Errors[ErrForbidden], fmt.Errorf("invalid role: %s not registered as a service role", requestClaims.Name)
	}

	method, ok := ctx.Value("request_method").(string)
	if !ok {
		return Errors[ErrForbidden], fmt.Errorf("could not parse request method")
	}

	if !slices.Contains(userClaims.AllowedMethods, method) {
		return Errors[ErrUnAuthorized], fmt.Errorf("not allowed to perform %s action", method)
	}

	route, ok := ctx.Value("request_route").(string)
	if !ok {
		return Errors[ErrForbidden], fmt.Errorf("could not parse request route")
	}

	for _, allowedRoute := range userClaims.AllowedRoutes {
		match, err := regexp.MatchString(allowedRoute, route)
		if err != nil {
			return Errors[ErrForbidden], fmt.Errorf("failed to match regex: %w", err)
		}

		if match {
			return nil, nil
		}
	}

	return Errors[ErrForbidden], fmt.Errorf("no metrics matched: default deny")
}

var roleMethod = map[Role][]string{
	RW: {
		http.MethodDelete,
		http.MethodGet,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
	},
	RO: {
		http.MethodGet,
	},
}

var endpointUserClaims = map[Role][]UserClaims{
	ADMIN: {
		{
			Name:           string(ADMIN),
			AllowedMethods: roleMethod[RW],
			AllowedRoutes: []string{
				".*",
			},
		},
	},
	RO: {
		{
			Name:           "DNSDIST-WORKER",
			AllowedMethods: roleMethod[RO],
			AllowedRoutes: []string{
				fmt.Sprintf("^%s$", routes.SPOOFS),
				fmt.Sprintf("^%s$", routes.SPOOFS_HASH),
			},
		},
	},
	RW: {
		{
			Name:           "OVERRIDER",
			AllowedMethods: roleMethod[RW],
			AllowedRoutes: []string{
				fmt.Sprintf("^%s$", routes.OVERRIDE),
			},
		},
		{
			Name:           "GSLB-OPERATOR",
			AllowedMethods: roleMethod[RW],
			AllowedRoutes: []string{
				fmt.Sprintf("^%s$", routes.SPOOFS),
				fmt.Sprintf("^%s/.*$", routes.SPOOFS),
			},
		},
	},
}
