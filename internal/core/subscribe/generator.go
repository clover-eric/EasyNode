package subscribe

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"easynode/internal/model"
)

func Link(n model.Node) string {
	if n.Protocol == "clash" {
		return ""
	}
	host := n.Host
	if host == "" {
		host = "127.0.0.1"
	}
	name := url.QueryEscape("EasyNode " + n.Protocol)
	switch n.Protocol {
	case "vless-reality":
		return fmt.Sprintf("vless://%s@%s:%d?encryption=none&security=reality&type=tcp&sni=www.microsoft.com&fp=chrome&pbk=%s&sid=%s#%s", n.UUID, host, n.Port, url.QueryEscape(n.RealityPublicKey), url.QueryEscape(n.RealityShortID), name)
	case "shadowsocks":
		user := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:" + n.Password))
		return fmt.Sprintf("ss://%s@%s:%d#%s", user, host, n.Port, name)
	case "trojan-tls":
		return fmt.Sprintf("trojan://%s@%s:%d?security=tls&type=tcp&sni=%s#%s", n.Password, host, n.Port, host, name)
	case "hysteria2":
		return fmt.Sprintf("hysteria2://%s@%s:%d?sni=%s#%s", n.Password, host, n.Port, host, name)
	case "vless-ws-tls":
		return fmt.Sprintf("vless://%s@%s:%d?encryption=none&security=tls&type=ws&path=/easynode&sni=%s#%s", n.UUID, host, n.Port, host, name)
	case "tuic":
		return fmt.Sprintf("tuic://%s:%s@%s:%d?congestion_control=bbr&sni=%s#%s", n.UUID, n.Password, host, n.Port, host, name)
	default:
		return ""
	}
}

func V2rayN(nodes []model.Node) string {
	var links []string
	for _, n := range nodes {
		if n.Status != "running" {
			continue
		}
		if link := Link(n); link != "" {
			links = append(links, link)
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
}

type clashProxy struct {
	name string
	yaml string
}

func clashProxies(nodes []model.Node) []clashProxy {
	out := []clashProxy{}
	for _, n := range nodes {
		if n.Status != "running" || n.Protocol == "clash" {
			continue
		}
		if n.Protocol == "vless-reality" && n.RealityPublicKey == "" {
			continue
		}
		if p, ok := clashProxyForNode(n); ok {
			out = append(out, p)
		}
	}
	return out
}

func clashProxyForNode(n model.Node) (clashProxy, bool) {
	host := n.Host
	if host == "" {
		host = "127.0.0.1"
	}
	name := "EasyNode " + n.Protocol
	var b strings.Builder
	b.WriteString("  - name: ")
	b.WriteString(clashQuote(name))
	b.WriteByte('\n')
	switch n.Protocol {
	case "shadowsocks":
		writeClashKV(&b, "type", "ss")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "cipher", "chacha20-ietf-poly1305")
		writeClashKV(&b, "password", n.Password)
		writeClashKV(&b, "udp", "true")
	case "vless-reality":
		writeClashKV(&b, "type", "vless")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "uuid", n.UUID)
		writeClashKV(&b, "network", "tcp")
		writeClashKV(&b, "tls", "true")
		writeClashKV(&b, "udp", "true")
		writeClashKV(&b, "servername", "www.microsoft.com")
		writeClashKV(&b, "client-fingerprint", "chrome")
		writeClashKV(&b, "skip-cert-verify", "false")
		b.WriteString("    reality-opts:\n")
		writeClashNestedKV(&b, "public-key", n.RealityPublicKey)
		writeClashNestedKV(&b, "short-id", n.RealityShortID)
	case "trojan-tls":
		writeClashKV(&b, "type", "trojan")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "password", n.Password)
		writeClashKV(&b, "sni", host)
		writeClashKV(&b, "udp", "true")
		writeClashKV(&b, "skip-cert-verify", "false")
	case "hysteria2":
		writeClashKV(&b, "type", "hysteria2")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "password", n.Password)
		writeClashKV(&b, "sni", host)
		writeClashKV(&b, "skip-cert-verify", "false")
	case "vless-ws-tls":
		writeClashKV(&b, "type", "vless")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "uuid", n.UUID)
		writeClashKV(&b, "network", "ws")
		writeClashKV(&b, "tls", "true")
		writeClashKV(&b, "udp", "true")
		writeClashKV(&b, "servername", host)
		writeClashKV(&b, "skip-cert-verify", "false")
		b.WriteString("    ws-opts:\n")
		writeClashNestedKV(&b, "path", "/easynode")
	case "tuic":
		writeClashKV(&b, "type", "tuic")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "uuid", n.UUID)
		writeClashKV(&b, "password", n.Password)
		writeClashKV(&b, "sni", host)
		writeClashKV(&b, "skip-cert-verify", "false")
		writeClashKV(&b, "congestion-controller", "bbr")
	default:
		return clashProxy{}, false
	}
	return clashProxy{name: name, yaml: b.String()}, true
}

func writeClashGroup(b *strings.Builder, name, typ string, proxies []string, extra string) {
	b.WriteString("  - name: ")
	b.WriteString(clashQuote(name))
	b.WriteByte('\n')
	b.WriteString("    type: ")
	b.WriteString(typ)
	b.WriteByte('\n')
	if extra != "" {
		b.WriteString(extra)
	}
	b.WriteString("    proxies:\n")
	for _, p := range proxies {
		b.WriteString("      - ")
		b.WriteString(clashQuote(p))
		b.WriteByte('\n')
	}
}

func writeClashKV(b *strings.Builder, key, value string) {
	b.WriteString("    ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(clashQuote(value))
	b.WriteByte('\n')
}

func writeClashNestedKV(b *strings.Builder, key, value string) {
	b.WriteString("      ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(clashQuote(value))
	b.WriteByte('\n')
}

func writeClashInt(b *strings.Builder, key string, value int) {
	b.WriteString("    ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(strconv.Itoa(value))
	b.WriteByte('\n')
}

func clashQuote(v string) string {
	if v == "true" || v == "false" {
		return v
	}
	return strconv.Quote(v)
}
