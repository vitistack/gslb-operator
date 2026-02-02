package jwt

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	mgrInstance *ServiceTokenManager
)

type ServiceTokenManager struct {
	issuer        *TokenIssuer
	userClaims    UserClaims
	expiresAt     time.Time
	current       string
	mu            sync.RWMutex
	renewalBuffer time.Duration // renew token before it expires
}

func InitServiceTokenManager(secret []byte, userClaimsName string, opts ...tokenIssuerOption) error {
	if mgrInstance != nil {
		return nil
	}
	issuer := NewTokenIssuer(secret, opts...)

	claims, ok := getUserClaims(userClaimsName)
	if !ok {
		return fmt.Errorf("userclaims not registered for: %s", userClaimsName)
	}

	mgrInstance = &ServiceTokenManager{
		issuer:        issuer,
		userClaims:    claims,
		mu:            sync.RWMutex{},
		renewalBuffer: time.Minute * 5,
	}
	return mgrInstance.refreshToken()
}

func GetInstance() *ServiceTokenManager {
	return mgrInstance
}

func (tm *ServiceTokenManager) GetServiceToken() (string, error) {
	if mgrInstance == nil {
		return "", jwt.ErrInvalidKey
	}

	tm.mu.RLock()
	needsRefresh := time.Now().Add(tm.renewalBuffer).After(tm.expiresAt)
	tm.mu.RUnlock()
	if needsRefresh {
		if err := tm.refreshToken(); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return "Bearer " + tm.current, nil
}

func (tm *ServiceTokenManager) GetSigningMethod() jwt.SigningMethod {
	return tm.issuer.signingMethod
}

func (tm *ServiceTokenManager) refreshToken() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.expiresAt = time.Now().Add(time.Hour * 24 * 30) // 30day expiration
	tm.userClaims.ExpiresAt = jwt.NewNumericDate(tm.expiresAt)
	tm.userClaims.IssuedAt = jwt.NewNumericDate(time.Now())
	tm.userClaims.Issuer = tm.userClaims.Name

	token, err := tm.issuer.New(tm.userClaims)
	if err != nil {
		return fmt.Errorf("could not generate token: %w", err)
	}
	tm.current = token

	return nil
}
