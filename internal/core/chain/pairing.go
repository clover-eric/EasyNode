package chain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"easynode/internal/model"
	"easynode/internal/util"
)

type Bundle struct {
	Code         string    `json:"code"`
	Endpoint     string    `json:"endpoint"`
	PublicKey    string    `json:"public_key"`
	OutboundLink string    `json:"outbound_link"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func NewCode(endpoint, outboundLink string) model.PairingCode {
	c := model.PairingCode{
		Code:      util.PairingCode(),
		Endpoint:  endpoint,
		PublicKey: util.Token(16),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	c.OutboundLink = outboundLink
	c.Bundle = EncodeBundle(c)
	return c
}

func EncodeBundle(c model.PairingCode) string {
	b, _ := json.Marshal(Bundle{Code: c.Code, Endpoint: c.Endpoint, PublicKey: c.PublicKey, OutboundLink: c.OutboundLink, ExpiresAt: c.ExpiresAt})
	return "ENPAIR-" + base64.RawURLEncoding.EncodeToString(b)
}

func DecodeBundle(raw string) (Bundle, error) {
	const prefix = "ENPAIR-"
	if len(raw) <= len(prefix) || raw[:len(prefix)] != prefix {
		return Bundle{}, errors.New("pairing token format invalid")
	}
	b, err := base64.RawURLEncoding.DecodeString(raw[len(prefix):])
	if err != nil {
		return Bundle{}, err
	}
	var out Bundle
	if err := json.Unmarshal(b, &out); err != nil {
		return Bundle{}, err
	}
	if out.Code == "" || out.OutboundLink == "" {
		return Bundle{}, errors.New("pairing token incomplete")
	}
	if !out.ExpiresAt.IsZero() && time.Now().After(out.ExpiresAt) {
		return Bundle{}, errors.New("pairing token expired")
	}
	return out, nil
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
