package infrastructure

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

type HMACVerifier struct {
	Secrets       map[string][]byte
	AllowUnsigned bool
}

func (v HMACVerifier) Verify(ctx context.Context, tenant string, content, signature []byte) error {
	secret, ok := v.Secrets[tenant]
	if !ok {
		if v.AllowUnsigned && len(signature) == 0 {
			return nil
		}
		return errors.New("tenant signing key not configured")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(content)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))
	if !hmac.Equal(expected, signature) {
		return fmt.Errorf("invalid module signature")
	}
	return nil
}
