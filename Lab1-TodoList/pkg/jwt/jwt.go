package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	ErrEmptySecret  = errors.New("jwt secret must not be empty")
	ErrInvalidToken = errors.New("invalid jwt token")
)

type Payload struct {
	Subject   string
	Issuer    string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Extra     map[string]interface{}
}

type claims struct {
	Extra map[string]interface{} `json:"extra,omitempty"`
	jwtv5.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

func NewManager(secret string, ttl time.Duration, issuer string) (*Manager, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt ttl must be positive: %s", ttl)
	}

	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
		issuer: issuer,
	}, nil
}

func (m *Manager) GenerateToken(subject string, extra map[string]interface{}) (string, error) {
	now := time.Now()
	tokenClaims := claims{
		Extra: cloneMap(extra),
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   subject,
			Issuer:    m.issuer,
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(m.ttl)),
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, tokenClaims)
	return token.SignedString(m.secret)
}

func (m *Manager) ValidateToken(tokenString string) error {
	_, err := m.parse(tokenString)
	return err
}

func (m *Manager) GetPayload(tokenString string) (Payload, error) {
	tokenClaims, err := m.parse(tokenString)
	if err != nil {
		return Payload{}, err
	}

	return Payload{
		Subject:   tokenClaims.Subject,
		Issuer:    tokenClaims.Issuer,
		ExpiresAt: tokenClaims.ExpiresAt.Time,
		IssuedAt:  tokenClaims.IssuedAt.Time,
		Extra:     cloneMap(tokenClaims.Extra),
	}, nil
}

func (m *Manager) parse(tokenString string) (*claims, error) {
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	tokenClaims := &claims{}
	token, err := jwtv5.ParseWithClaims(tokenString, tokenClaims, func(token *jwtv5.Token) (interface{}, error) {
		if token.Method != jwtv5.SigningMethodHS256 {
			return nil, fmt.Errorf("%w: unexpected signing method %q", ErrInvalidToken, token.Method.Alg())
		}
		return m.secret, nil
	}, jwtv5.WithIssuer(m.issuer))
	if err != nil || !token.Valid {
		if err == nil {
			err = ErrInvalidToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return tokenClaims, nil
}

func cloneMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
