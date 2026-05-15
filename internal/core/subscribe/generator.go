package subscribe

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"easynode/internal/model"
)

func Link(n model.Node) string {
	host := n.Host
	if host == "" {
		host = "127.0.0.1"
	}
	name := url.QueryEscape("EasyNode " + n.Protocol)
	switch n.Protocol {
	case "vless-reality":
		return fmt.Sprintf("vless://%s@%s:%d?encryption=none&security=reality&flow=xtls-rprx-vision&type=tcp&sni=www.microsoft.com&fp=chrome&pbk=%s&sid=%s#%s", n.UUID, host, n.Port, url.QueryEscape(n.RealityPublicKey), url.QueryEscape(n.RealityShortID), name)
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
		if n.Status == "running" {
			links = append(links, Link(n))
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
}
