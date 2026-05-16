package singbox

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"easynode/internal/model"
)

func TestGenerateRealityMaterialFallback(t *testing.T) {
	m := GenerateRealityMaterial()
	if m.PrivateKey == "" || m.PublicKey == "" || m.ShortID == "" {
		t.Fatalf("reality material should be complete: %#v", m)
	}
	private, err := base64.RawURLEncoding.DecodeString(m.PrivateKey)
	if err != nil {
		t.Fatalf("private key should use raw URL base64: %v", err)
	}
	public, err := base64.RawURLEncoding.DecodeString(m.PublicKey)
	if err != nil {
		t.Fatalf("public key should use raw URL base64: %v", err)
	}
	if len(private) != 32 || len(public) != 32 {
		t.Fatalf("reality keys should be 32-byte X25519 keys, got %d/%d", len(private), len(public))
	}
}

func TestWriteConfigIncludesShadowsocksInboundWithoutCert(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteConfig(dir, []model.Node{{
		ID:       "shadowsocks",
		Protocol: "shadowsocks",
		Status:   "running",
		Port:     8388,
		Password: "secret",
	}}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{`"type": "shadowsocks"`, `"listen_port": 8388`, `"method": "chacha20-ietf-poly1305"`} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %s:\n%s", want, cfg)
		}
	}
}

func TestX25519Vector(t *testing.T) {
	private, _ := hex.DecodeString("a546e36bf0527c9d3b16154b82465edd62144c0ac1fc5a18506a2244ba449ac4")
	point, _ := hex.DecodeString("e6db6867583030db3594c1a424b15f7c726624ec26b3353b10a903a6d1ab1c4c")
	public := x25519(private, point)
	got := hex.EncodeToString(public)
	want := "384b289c8a7748eea3b4a3e4d8c8733854403e3be7355c448c101136ead8fd58"
	if got != want {
		t.Fatalf("unexpected X25519 public key: got %s want %s", got, want)
	}
}
