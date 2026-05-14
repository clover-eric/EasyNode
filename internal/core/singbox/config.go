package singbox

import (
	"encoding/json"
	"os"
	"path/filepath"

	"easynode/internal/model"
)

func WriteConfig(dataDir string, nodes []model.Node, peers []model.ChainPeer) (string, error) {
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
		inbounds = append(inbounds, inbound(n))
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

func inbound(n model.Node) map[string]any {
	base := map[string]any{"type": protocolType(n.Protocol), "tag": n.ID, "listen": "::", "listen_port": n.Port}
	switch n.Protocol {
	case "vless-reality":
		base["users"] = []any{map[string]any{"uuid": n.UUID, "flow": "xtls-rprx-vision"}}
		base["tls"] = map[string]any{"enabled": true, "server_name": "www.microsoft.com", "reality": map[string]any{"enabled": true, "handshake": map[string]any{"server": "www.microsoft.com", "server_port": 443}}}
	case "trojan-tls":
		base["users"] = []any{map[string]any{"password": n.Password}}
		base["tls"] = map[string]any{"enabled": true}
	case "hysteria2":
		base["users"] = []any{map[string]any{"password": n.Password}}
		base["tls"] = map[string]any{"enabled": true}
	case "tuic":
		base["users"] = []any{map[string]any{"uuid": n.UUID, "password": n.Password}}
		base["tls"] = map[string]any{"enabled": true}
	case "vless-ws-tls":
		base["users"] = []any{map[string]any{"uuid": n.UUID}}
		base["transport"] = map[string]any{"type": "ws", "path": "/easynode"}
		base["tls"] = map[string]any{"enabled": true}
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
