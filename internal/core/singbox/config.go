package singbox

import (
	"encoding/json"
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
		if n.Protocol != "vless-reality" && (certPath == "" || keyPath == "") {
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
	return path, os.WriteFile(path, b, 0600)
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
			out["flow"] = values.Get("flow")
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
