package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func (t *tokenizer) sign(c claims) (string, error) {
	payloadStr, err := utils.FromObjectToStringJSON(c)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPayloadEncode, err)
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payloadStr))
	signature := base64.RawURLEncoding.EncodeToString(t.mac(encoded))
	return encoded + delimiter + signature, nil
}

func (t *tokenizer) verify(token string) (claims, error) {
	encoded, signature, ok := strings.Cut(token, delimiter)
	if !ok || encoded == "" || signature == "" || strings.Contains(signature, delimiter) {
		return claims{}, ErrTokenInvalid
	}

	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return claims{}, ErrTokenInvalid
	}
	if !hmac.Equal(got, t.mac(encoded)) {
		return claims{}, ErrTokenInvalid
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return claims{}, ErrTokenInvalid
	}
	c, err := utils.FromStringToObjectJSON[claims](string(raw))
	if err != nil {
		return claims{}, ErrTokenInvalid
	}
	if !utils.ValidateULID(c.ID) || c.ExpiresAt <= 0 || c.Payload == nil {
		return claims{}, ErrTokenInvalid
	}
	now, err := t.now()
	if err != nil {
		return claims{}, err
	}
	if now.Unix() >= c.ExpiresAt {
		return claims{}, ErrTokenExpired
	}
	return c, nil
}

func (t *tokenizer) mac(data string) []byte {
	m := hmac.New(sha256.New, t.secret)
	m.Write([]byte(data))
	return m.Sum(nil)
}
