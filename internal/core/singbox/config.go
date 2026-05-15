package singbox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "privatekey:") || strings.HasPrefix(strings.ToLower(line), "private_key:") {
			m.PrivateKey = strings.TrimSpace(line[strings.Index(line, ":")+1:])
		}
		if strings.HasPrefix(strings.ToLower(line), "publickey:") || strings.HasPrefix(strings.ToLower(line), "public_key:") {
			m.PublicKey = strings.TrimSpace(line[strings.Index(line, ":")+1:])
		}
	}
	return m
}

func RestartService() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	return exec.Command("systemctl", "restart", "easynode-singbox").Run()
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
	inbounds := make([]any, 0, len(nodes))
	for _, n := range nodes {
		if n.Status != "running" {
			continue
		}
		if n.Protocol != "vless-reality" && (certPath == "" || keyPath == "") {
			continue
		}
		inbounds = append(inbounds, inbound(n, certPath, keyPath))
	}
	cfg["inbounds"] = inbounds
	if len(peers) > 0 {
		cfg["route"] = map[string]any{"final": "direct"}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dataDir, "sing-box.json")
	return path, os.WriteFile(path, b, 0600)
}

func inbound(n model.Node, certPath, keyPath string) map[string]any {
	base := map[string]any{"type": protocolType(n.Protocol), "tag": n.ID, "listen": "::", "listen_port": n.Port}
	switch n.Protocol {
	case "vless-reality":
		base["users"] = []any{map[string]any{"uuid": n.UUID, "flow": "xtls-rprx-vision"}}
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
