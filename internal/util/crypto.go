package util

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

func Token(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func UUID() string {
	s := Token(16)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

func PairingCode() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
	if len(s) > 6 {
		s = s[:6]
	}
	return s[:3] + "-" + s[3:]
}
