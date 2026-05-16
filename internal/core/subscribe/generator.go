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
		if n.Status != "running" {
			continue
		}
		if link := Link(n); link != "" {
			links = append(links, link)
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
}

func Clash(nodes []model.Node) string {
	proxies := clashProxies(nodes)
	if len(proxies) == 0 {
		return "mixed-port: 7890\nallow-lan: true\nmode: rule\nlog-level: warning\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"
	}

	names := make([]string, 0, len(proxies))
	for _, p := range proxies {
		names = append(names, p.name)
	}

	var b strings.Builder
	b.WriteString("mixed-port: 7890\n")
	b.WriteString("allow-lan: true\n")
	b.WriteString("mode: rule\n")
	b.WriteString("log-level: warning\n")
	b.WriteString("ipv6: true\n")
	b.WriteString("unified-delay: true\n")
	b.WriteString("tcp-concurrent: true\n")
	b.WriteString("find-process-mode: strict\n")
	b.WriteString("global-client-fingerprint: chrome\n")
	b.WriteString("profile:\n  store-selected: true\n  store-fake-ip: true\n")
	b.WriteString("sniffer:\n  enable: true\n  sniff:\n    TLS:\n      ports: [443, 8443]\n    HTTP:\n      ports: [80, 8080-8880]\n      override-destination: true\n")
	b.WriteString("dns:\n  enable: true\n  listen: 0.0.0.0:1053\n  ipv6: true\n  enhanced-mode: fake-ip\n  fake-ip-range: 198.18.0.1/16\n  fake-ip-filter:\n    - '*.lan'\n    - '*.local'\n    - 'localhost.ptlogin2.qq.com'\n  default-nameserver:\n    - 223.5.5.5\n    - 119.29.29.29\n  nameserver:\n    - https://223.5.5.5/dns-query\n    - https://doh.pub/dns-query\n  fallback:\n    - https://1.1.1.1/dns-query\n    - https://8.8.8.8/dns-query\n  fallback-filter:\n    geoip: true\n    geoip-code: CN\n")
	b.WriteString("proxies:\n")
	for _, p := range proxies {
		b.WriteString(p.yaml)
	}
	b.WriteString("proxy-groups:\n")
	writeClashGroup(&b, "Proxy", "select", append([]string{"Auto", "Fallback"}, append(names, "DIRECT")...), "")
	writeClashGroup(&b, "Auto", "url-test", names, "    url: http://www.gstatic.com/generate_204\n    interval: 300\n    tolerance: 50\n    lazy: true\n")
	writeClashGroup(&b, "Fallback", "fallback", names, "    url: http://www.gstatic.com/generate_204\n    interval: 300\n    lazy: true\n")
	writeClashGroup(&b, "Global", "select", []string{"Proxy", "Auto", "Fallback"}, "")
	writeClashGroup(&b, "China", "select", append([]string{"DIRECT"}, names...), "")
	b.WriteString("rules:\n")
	b.WriteString("  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve\n")
	b.WriteString("  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve\n")
	b.WriteString("  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve\n")
	b.WriteString("  - GEOIP,private,DIRECT,no-resolve\n")
	b.WriteString("  - DOMAIN-SUFFIX,google.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,googleapis.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,gstatic.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,youtube.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,ytimg.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,googlevideo.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,telegram.org,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,t.me,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,netflix.com,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,nflxvideo.net,Global\n")
	b.WriteString("  - DOMAIN-SUFFIX,cn,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,qq.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,taobao.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,alipay.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,baidu.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,bilibili.com,China\n")
	b.WriteString("  - GEOIP,cn,China,no-resolve\n")
	b.WriteString("  - MATCH,Proxy\n")
	return b.String()
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
	case "vless-reality":
		writeClashKV(&b, "type", "vless")
		writeClashKV(&b, "server", host)
		writeClashInt(&b, "port", n.Port)
		writeClashKV(&b, "uuid", n.UUID)
		writeClashKV(&b, "network", "tcp")
		writeClashKV(&b, "tls", "true")
		writeClashKV(&b, "udp", "true")
		writeClashKV(&b, "flow", "xtls-rprx-vision")
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
