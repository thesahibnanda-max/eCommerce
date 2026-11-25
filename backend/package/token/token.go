package token

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	delimiter        = ":"
	minSecretLength  = 32
	usedTokenCache   = 100
	accessTokenTTL   = 15 * time.Minute
	refreshTokenTTL  = 24 * time.Hour
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

type claims struct {
	ID        string         `json:"jti"`
	Type      string         `json:"typ"`
	IssuedAt  int64          `json:"iat"`
	ExpiresAt int64          `json:"exp"`
	Payload   map[string]any `json:"payload"`
}

type Tokenizer interface {
	GenerateNewTokens(payload map[string]any) (accessToken string, refreshToken string, err error)
	VerifyAccessToken(accessToken string) (map[string]any, error)
	RefreshAccessToken(refreshToken string) (accessToken string, newRefreshToken string, err error)
}

type tokenizer struct {
	usedRefreshTokens *lru.Cache[string, struct{}]
	secret            []byte
	now               func() (time.Time, error)
}

func NewTokenizer(secret string) (Tokenizer, error) {
	if len(secret) < minSecretLength {
		return nil, ErrSecretTooShort
	}
	used, err := lru.New[string, struct{}](usedTokenCache)
	if err != nil {
		return nil, err
	}
	return &tokenizer{
		usedRefreshTokens: used,
		secret:            []byte(secret),
		now:               utils.TimeNow,
	}, nil
}

func (t *tokenizer) GenerateNewTokens(payload map[string]any) (string, string, error) {
	if payload == nil {
		return "", "", ErrPayloadInvalid
	}

	now, err := t.now()
	if err != nil {
		return "", "", err
	}

	accessToken, err := t.sign(claims{
		ID:        utils.NewULID(),
		Type:      tokenTypeAccess,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(accessTokenTTL).Unix(),
		Payload:   payload,
	})
	if err != nil {
		return "", "", err
	}

	refreshToken, err := t.sign(claims{
		ID:        utils.NewULID(),
		Type:      tokenTypeRefresh,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(refreshTokenTTL).Unix(),
		Payload:   payload,
	})
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (t *tokenizer) VerifyAccessToken(accessToken string) (map[string]any, error) {
	c, err := t.verify(accessToken)
	if err != nil {
		return nil, err
	}
	if c.Type != tokenTypeAccess {
		return nil, ErrTokenWrongType
	}
	return c.Payload, nil
}

func (t *tokenizer) RefreshAccessToken(refreshToken string) (string, string, error) {
	c, err := t.verify(refreshToken)
	if err != nil {
		return "", "", err
	}
	if c.Type != tokenTypeRefresh {
		return "", "", ErrTokenWrongType
	}
	if used, _ := t.usedRefreshTokens.ContainsOrAdd(c.ID, struct{}{}); used {
		return "", "", ErrTokenReused
	}
	return t.GenerateNewTokens(c.Payload)
}
