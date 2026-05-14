package chain

import (
	"errors"
	"time"

	"easynode/internal/model"
	"easynode/internal/util"
)

func NewCode(endpoint string) model.PairingCode {
	return model.PairingCode{
		Code:      util.PairingCode(),
		Endpoint:  endpoint,
		PublicKey: util.Token(16),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func Pair(codes []model.PairingCode, code, endpoint, publicKey string) (model.PairingCode, error) {
	now := time.Now()
	for _, c := range codes {
		if c.Code == code && !c.Used && c.ExpiresAt.After(now) {
			if endpoint != "" {
				c.Endpoint = endpoint
			}
			if publicKey != "" {
				c.PublicKey = publicKey
			}
			return c, nil
		}
	}
	return model.PairingCode{}, errors.New("pairing code invalid or expired")
}
