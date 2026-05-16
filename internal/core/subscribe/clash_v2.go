package subscribe

import (
	"strings"

	"easynode/internal/model"
)

type ClashConfig struct {
	Nodes       []model.Node
	UseRuleSet  bool
	RuleSetBase string
}

func ClashV2(nodes []model.Node) string {
	cfg := ClashConfig{
		Nodes:       nodes,
		UseRuleSet:  true,
		RuleSetBase: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release",
	}
	return cfg.Generate()
}

func (c *ClashConfig) Generate() string {
	proxies := clashProxies(c.Nodes)
	if len(proxies) == 0 {
		return c.emptyConfig()
	}

	var b strings.Builder
	c.writeGlobal(&b)
	c.writeDNS(&b)
	c.writeSniffer(&b)
	c.writeProxies(&b, proxies)
	c.writeProxyGroups(&b, proxies)
	if c.UseRuleSet {
		c.writeRuleProviders(&b)
	}
	c.writeRules(&b)
	return b.String()
}

func (c *ClashConfig) emptyConfig() string {
	return `mixed-port: 7890
allow-lan: true
mode: rule
log-level: warning
ipv6: true
proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`
}

func (c *ClashConfig) writeGlobal(b *strings.Builder) {
	b.WriteString(`mixed-port: 7890
allow-lan: true
bind-address: '*'
mode: rule
log-level: warning
ipv6: true
unified-delay: true
tcp-concurrent: true
find-process-mode: strict
global-client-fingerprint: chrome
profile:
  store-selected: true
  store-fake-ip: true
geodata-mode: true
geox-url:
  geoip: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb"
`)
}

func (c *ClashConfig) writeDNS(b *strings.Builder) {
	b.WriteString("dns:\n")
	b.WriteString("  enable: true\n")
	b.WriteString("  listen: 0.0.0.0:1053\n")
	b.WriteString("  ipv6: true\n")
	b.WriteString("  enhanced-mode: fake-ip\n")
	b.WriteString("  fake-ip-range: 198.18.0.1/16\n")
	b.WriteString("  fake-ip-filter:\n")
	b.WriteString("    - '*.lan'\n")
	b.WriteString("    - '*.local'\n")
	b.WriteString("    - '+.msftconnecttest.com'\n")
	b.WriteString("    - '+.msftncsi.com'\n")
	b.WriteString("    - 'localhost.ptlogin2.qq.com'\n")
	b.WriteString("  default-nameserver:\n")
	b.WriteString("    - 223.5.5.5\n")
	b.WriteString("    - 119.29.29.29\n")
	b.WriteString("  nameserver:\n")
	b.WriteString("    - https://doh.pub/dns-query\n")
	b.WriteString("    - https://223.5.5.5/dns-query\n")
	b.WriteString("  proxy-server-nameserver:\n")
	b.WriteString("    - https://223.5.5.5/dns-query\n")
	b.WriteString("  nameserver-policy:\n")
	b.WriteString("    'geosite:cn':\n")
	b.WriteString("      - https://doh.pub/dns-query\n")
	b.WriteString("      - https://223.5.5.5/dns-query\n")
	b.WriteString("    'geosite:geolocation-!cn':\n")
	b.WriteString("      - https://1.1.1.1/dns-query\n")
	b.WriteString("      - https://8.8.8.8/dns-query\n")
}

func (c *ClashConfig) writeSniffer(b *strings.Builder) {
	b.WriteString("sniffer:\n")
	b.WriteString("  enable: true\n")
	b.WriteString("  sniff:\n")
	b.WriteString("    TLS:\n")
	b.WriteString("      ports: [443, 8443]\n")
	b.WriteString("    HTTP:\n")
	b.WriteString("      ports: [80, 8080-8880]\n")
	b.WriteString("      override-destination: true\n")
	b.WriteString("    QUIC:\n")
	b.WriteString("      ports: [443, 8443]\n")
}

func (c *ClashConfig) writeProxies(b *strings.Builder, proxies []clashProxy) {
	b.WriteString("proxies:\n")
	for _, p := range proxies {
		b.WriteString(p.yaml)
	}
}

func (c *ClashConfig) writeProxyGroups(b *strings.Builder, proxies []clashProxy) {
	names := make([]string, 0, len(proxies))
	for _, p := range proxies {
		names = append(names, p.name)
	}

	b.WriteString("proxy-groups:\n")
	writeClashGroup(b, "GLOBAL", "select", []string{"Proxy", "China", "DIRECT"}, "")
	writeClashGroup(b, "Proxy", "select", append([]string{"Auto", "Fallback", "LoadBalance"}, names...), "")
	writeClashGroup(b, "Auto", "url-test", names, "    url: http://www.gstatic.com/generate_204\n    interval: 300\n    tolerance: 50\n    lazy: true\n")
	writeClashGroup(b, "Fallback", "fallback", names, "    url: http://www.gstatic.com/generate_204\n    interval: 300\n    lazy: true\n")
	writeClashGroup(b, "LoadBalance", "load-balance", names, "    url: http://www.gstatic.com/generate_204\n    interval: 300\n    strategy: consistent-hashing\n    lazy: true\n")
	writeClashGroup(b, "Streaming", "select", append([]string{"Proxy", "Auto"}, names...), "")
	writeClashGroup(b, "China", "select", []string{"DIRECT", "Proxy"}, "")
	writeClashGroup(b, "AdBlock", "select", []string{"REJECT", "DIRECT", "Proxy"}, "")
}

func (c *ClashConfig) writeRuleProviders(b *strings.Builder) {
	base := c.RuleSetBase
	b.WriteString("rule-providers:\n")
	providers := []struct {
		name     string
		behavior string
		file     string
	}{
		{"reject", "domain", "reject.txt"},
		{"icloud", "domain", "icloud.txt"},
		{"apple", "domain", "apple.txt"},
		{"google", "domain", "google.txt"},
		{"proxy", "domain", "proxy.txt"},
		{"direct", "domain", "direct.txt"},
		{"private", "domain", "private.txt"},
		{"gfw", "domain", "gfw.txt"},
		{"tld-not-cn", "domain", "tld-not-cn.txt"},
		{"telegramcidr", "ipcidr", "telegramcidr.txt"},
		{"cncidr", "ipcidr", "cncidr.txt"},
		{"lancidr", "ipcidr", "lancidr.txt"},
	}
	for _, p := range providers {
		b.WriteString("  ")
		b.WriteString(p.name)
		b.WriteString(":\n    type: http\n    behavior: ")
		b.WriteString(p.behavior)
		b.WriteString("\n    url: \"")
		b.WriteString(base)
		b.WriteString("/")
		b.WriteString(p.file)
		b.WriteString("\"\n    path: ./ruleset/")
		b.WriteString(p.name)
		b.WriteString(".yaml\n    interval: 86400\n")
	}
}

func (c *ClashConfig) writeRules(b *strings.Builder) {
	b.WriteString("rules:\n")
	if c.UseRuleSet {
		c.writeRuleSetRules(b)
	} else {
		c.writeSimpleRules(b)
	}
}

func (c *ClashConfig) writeRuleSetRules(b *strings.Builder) {
	b.WriteString("  - RULE-SET,private,DIRECT\n")
	b.WriteString("  - RULE-SET,lancidr,DIRECT,no-resolve\n")
	b.WriteString("  - RULE-SET,reject,AdBlock\n")
	b.WriteString("  - RULE-SET,icloud,DIRECT\n")
	b.WriteString("  - RULE-SET,apple,DIRECT\n")
	b.WriteString("  - RULE-SET,google,Proxy\n")
	b.WriteString("  - RULE-SET,telegramcidr,Proxy,no-resolve\n")
	b.WriteString("  - DOMAIN-SUFFIX,netflix.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,nflxvideo.net,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,youtube.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,ytimg.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,googlevideo.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,spotify.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,twitch.tv,Streaming\n")
	b.WriteString("  - RULE-SET,proxy,Proxy\n")
	b.WriteString("  - RULE-SET,gfw,Proxy\n")
	b.WriteString("  - RULE-SET,tld-not-cn,Proxy\n")
	b.WriteString("  - RULE-SET,direct,China\n")
	b.WriteString("  - GEOIP,cn,China,no-resolve\n")
	b.WriteString("  - RULE-SET,cncidr,China,no-resolve\n")
	b.WriteString("  - MATCH,GLOBAL\n")
}

func (c *ClashConfig) writeSimpleRules(b *strings.Builder) {
	b.WriteString("  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve\n")
	b.WriteString("  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve\n")
	b.WriteString("  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve\n")
	b.WriteString("  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve\n")
	b.WriteString("  - GEOIP,private,DIRECT,no-resolve\n")
	b.WriteString("  - DOMAIN-SUFFIX,netflix.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,youtube.com,Streaming\n")
	b.WriteString("  - DOMAIN-SUFFIX,google.com,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,googleapis.com,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,telegram.org,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,t.me,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,twitter.com,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,facebook.com,Proxy\n")
	b.WriteString("  - DOMAIN-SUFFIX,cn,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,qq.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,baidu.com,China\n")
	b.WriteString("  - DOMAIN-SUFFIX,bilibili.com,China\n")
	b.WriteString("  - GEOIP,cn,China,no-resolve\n")
	b.WriteString("  - MATCH,GLOBAL\n")
}



