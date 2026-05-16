package singbox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"easynode/internal/model"
	"easynode/internal/util"
)

type RealityMaterial struct {
	PrivateKey string
	PublicKey  string
	ShortID    string
}

func GenerateRealityMaterial() RealityMaterial {
	m := RealityMaterial{ShortID: util.Token(8)}
	out, err := exec.Command("sing-box", "generate", "reality-keypair").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			switch normalizeKeyName(key) {
			case "privatekey":
				m.PrivateKey = strings.TrimSpace(value)
			case "publickey":
				m.PublicKey = strings.TrimSpace(value)
			}
		}
	}
	if m.PrivateKey != "" && m.PublicKey != "" {
		return m
	}
	private := make([]byte, 32)
	if _, err := rand.Read(private); err != nil {
		return m
	}
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64
	public := x25519Basepoint(private)
	m.PrivateKey = base64.RawURLEncoding.EncodeToString(private)
	m.PublicKey = base64.RawURLEncoding.EncodeToString(public)
	return m
}

func normalizeKeyName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "_", "")
	v = strings.ReplaceAll(v, "-", "")
	v = strings.ReplaceAll(v, " ", "")
	return v
}

func x25519Basepoint(private []byte) []byte {
	basepoint := make([]byte, 32)
	basepoint[0] = 9
	return x25519(private, basepoint)
}

func x25519(private, point []byte) []byte {
	p := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	a24 := big.NewInt(121666)
	k := append([]byte(nil), private...)
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
	x1 := leBytesToInt(point)
	x2, z2 := big.NewInt(1), big.NewInt(0)
	x3, z3 := new(big.Int).Set(x1), big.NewInt(1)
	swap := 0
	for t := 254; t >= 0; t-- {
		kt := int((k[t/8] >> uint(t&7)) & 1)
		swap ^= kt
		conditionalSwap(swap, x2, x3)
		conditionalSwap(swap, z2, z3)
		swap = kt

		d := modSub(x3, z3, p)
		b := modSub(x2, z2, p)
		a := modAdd(x2, z2, p)
		c := modAdd(x3, z3, p)
		da := modMul(d, a, p)
		cb := modMul(c, b, p)
		bb := modMul(b, b, p)
		aa := modMul(a, a, p)
		x3 = modMul(modAdd(da, cb, p), modAdd(da, cb, p), p)
		z3 = modMul(x1, modMul(modSub(da, cb, p), modSub(da, cb, p), p), p)
		x2 = modMul(aa, bb, p)
		e := modSub(aa, bb, p)
		z2 = modMul(e, modAdd(bb, modMul(a24, e, p), p), p)
	}
	conditionalSwap(swap, x2, x3)
	conditionalSwap(swap, z2, z3)
	out := modMul(x2, new(big.Int).ModInverse(z2, p), p)
	return intToLEBytes(out, 32)
}

func conditionalSwap(swap int, a, b *big.Int) {
	if swap == 1 {
		tmp := new(big.Int).Set(a)
		a.Set(b)
		b.Set(tmp)
	}
}

func modAdd(a, b, p *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Add(a, b), p)
}

func modSub(a, b, p *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Sub(a, b), p)
}

func modMul(a, b, p *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Mul(a, b), p)
}

func leBytesToInt(in []byte) *big.Int {
	be := make([]byte, len(in))
	for i := range in {
		be[len(in)-1-i] = in[i]
	}
	return new(big.Int).SetBytes(be)
}

func intToLEBytes(v *big.Int, size int) []byte {
	be := v.Bytes()
	out := make([]byte, size)
	for i := 0; i < len(be) && i < size; i++ {
		out[i] = be[len(be)-1-i]
	}
	return out
}

func RestartService() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	return exec.Command("systemctl", "restart", "easynode-singbox").Run()
}

func ValidateConfig(path string) error {
	singBox, err := exec.LookPath("sing-box")
	if err != nil {
		return nil
	}
	out, err := exec.Command(singBox, "check", "-c", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box config invalid: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func WriteConfig(dataDir string, nodes []model.Node, peers []model.ChainPeer, certPath, keyPath string) (string, error) {
	cfg := map[string]any{
		"log":      map[string]any{"level": "info"},
		"inbounds": []any{},
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"experimental": map[string]any{
			"cache_file": map[string]any{"enabled": true},
		},
	}
	chainTag := ""
	outbounds := []any{map[string]any{"type": "direct", "tag": "direct"}}
	if len(peers) > 0 {
		if out := chainOutbound(peers[0]); out != nil {
			chainTag = "chain-exit-" + peers[0].ID
			out["tag"] = chainTag
			outbounds = append(outbounds, out)
		}
	}
	cfg["outbounds"] = outbounds
	inbounds := make([]any, 0, len(nodes))
	for _, n := range nodes {
		if n.Status != "running" {
			continue
		}
		if n.Protocol == "clash" {
			continue
		}
		if n.Protocol != "vless-reality" && n.Protocol != "shadowsocks" && (certPath == "" || keyPath == "") {
			continue
		}
		inbounds = append(inbounds, inbound(n, certPath, keyPath))
	}
	cfg["inbounds"] = inbounds
	if chainTag != "" {
		cfg["route"] = map[string]any{"final": chainTag}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dataDir, "sing-box.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return "", err
	}
	if err := ValidateConfig(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func chainOutbound(peer model.ChainPeer) map[string]any {
	u, err := url.Parse(peer.OutboundLink)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	port, _ := strconv.Atoi(u.Port())
	values := u.Query()
	switch u.Scheme {
	case "vless":
		out := map[string]any{
			"type":        "vless",
			"server":      u.Hostname(),
			"server_port": port,
			"uuid":        strings.TrimPrefix(u.User.String(), ""),
		}
		if values.Get("security") == "reality" {
			if values.Get("flow") != "" {
				out["flow"] = values.Get("flow")
			}
			out["tls"] = map[string]any{
				"enabled":     true,
				"server_name": values.Get("sni"),
				"utls":        map[string]any{"enabled": true, "fingerprint": valueOr(values.Get("fp"), "chrome")},
				"reality": map[string]any{
					"enabled":    true,
					"public_key": values.Get("pbk"),
					"short_id":   values.Get("sid"),
				},
			}
		} else if values.Get("security") == "tls" {
			out["tls"] = map[string]any{"enabled": true, "server_name": values.Get("sni")}
		}
		if values.Get("type") == "ws" {
			out["transport"] = map[string]any{"type": "ws", "path": valueOr(values.Get("path"), "/")}
		}
		return out
	case "trojan":
		out := map[string]any{"type": "trojan", "server": u.Hostname(), "server_port": port}
		if password, ok := u.User.Password(); ok {
			out["password"] = password
		} else {
			out["password"] = u.User.Username()
		}
		out["tls"] = map[string]any{"enabled": true, "server_name": values.Get("sni")}
		return out
	case "hysteria2":
		password := u.User.Username()
		out := map[string]any{"type": "hysteria2", "server": u.Hostname(), "server_port": port, "password": password}
		out["tls"] = map[string]any{"enabled": true, "server_name": values.Get("sni")}
		return out
	case "ss":
		user := u.User.Username()
		if decoded, err := base64.RawURLEncoding.DecodeString(user); err == nil {
			user = string(decoded)
		}
		method, password, ok := strings.Cut(user, ":")
		if !ok {
			return nil
		}
		return map[string]any{"type": "shadowsocks", "server": u.Hostname(), "server_port": port, "method": method, "password": password}
	default:
		return nil
	}
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func inbound(n model.Node, certPath, keyPath string) map[string]any {
	base := map[string]any{"type": protocolType(n.Protocol), "tag": n.ID, "listen": "::", "listen_port": n.Port}
	switch n.Protocol {
	case "shadowsocks":
		base["method"] = "chacha20-ietf-poly1305"
		base["password"] = n.Password
	case "vless-reality":
		base["users"] = []any{map[string]any{"uuid": n.UUID}}
		base["tls"] = map[string]any{
			"enabled":     true,
			"server_name": "www.microsoft.com",
			"reality": map[string]any{
				"enabled":     true,
				"private_key": n.RealityPrivateKey,
				"short_id":    []string{n.RealityShortID},
				"handshake":   map[string]any{"server": "www.microsoft.com", "server_port": 443},
			},
		}
	case "trojan-tls":
		base["users"] = []any{map[string]any{"password": n.Password}}
		base["tls"] = map[string]any{"enabled": true, "certificate_path": certPath, "key_path": keyPath}
	case "hysteria2":
		base["users"] = []any{map[string]any{"password": n.Password}}
		base["tls"] = map[string]any{"enabled": true, "certificate_path": certPath, "key_path": keyPath}
	case "tuic":
		base["users"] = []any{map[string]any{"uuid": n.UUID, "password": n.Password}}
		base["tls"] = map[string]any{"enabled": true, "certificate_path": certPath, "key_path": keyPath}
	case "vless-ws-tls":
		base["users"] = []any{map[string]any{"uuid": n.UUID}}
		base["transport"] = map[string]any{"type": "ws", "path": "/easynode"}
		base["tls"] = map[string]any{"enabled": true, "certificate_path": certPath, "key_path": keyPath}
	}
	return base
}

func protocolType(p string) string {
	switch p {
	case "vless-reality", "vless-ws-tls":
		return "vless"
	case "shadowsocks":
		return "shadowsocks"
	case "trojan-tls":
		return "trojan"
	case "hysteria2":
		return "hysteria2"
	case "tuic":
		return "tuic"
	default:
		return p
	}
}
