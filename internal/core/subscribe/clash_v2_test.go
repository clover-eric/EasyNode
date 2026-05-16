package subscribe

import (
	"strings"
	"testing"

	"easynode/internal/model"
	"easynode/internal/util"
)

func TestClashV2GeneratesValidYAML(t *testing.T) {
	nodes := []model.Node{
		{
			ID: "ss", Protocol: "shadowsocks", Status: "running",
			Port: 8388, Password: "secret", Host: "1.2.3.4",
		},
		{
			ID: "vless", Protocol: "vless-reality", Status: "running",
			Port: 443, UUID: util.UUID(), Host: "1.2.3.4",
			RealityPrivateKey: "priv", RealityPublicKey: "pub", RealityShortID: "abcd",
		},
	}

	yaml := ClashV2(nodes)

	// 验证基本结构
	required := []string{
		"mixed-port: 7890",
		"allow-lan: true",
		"mode: rule",
		"dns:",
		"proxies:",
		"proxy-groups:",
		"rules:",
		"rule-providers:",
	}

	for _, s := range required {
		if !strings.Contains(yaml, s) {
			t.Errorf("missing required field: %s", s)
		}
	}

	// 验证代理组
	groups := []string{"GLOBAL", "Proxy", "Auto", "Fallback", "LoadBalance", "Streaming", "China", "AdBlock"}
	for _, g := range groups {
		if !strings.Contains(yaml, `name: "`+g+`"`) && !strings.Contains(yaml, "name: "+g) {
			t.Errorf("missing proxy group: %s", g)
		}
	}

	// 验证规则集
	ruleSets := []string{"reject", "direct", "proxy", "gfw", "cncidr"}
	for _, rs := range ruleSets {
		if !strings.Contains(yaml, rs+":") {
			t.Errorf("missing rule-set: %s", rs)
		}
	}

	// 验证 DNS 分流
	if !strings.Contains(yaml, "nameserver-policy:") {
		t.Error("missing DNS nameserver-policy")
	}
	if !strings.Contains(yaml, "geosite:cn") {
		t.Error("missing geosite:cn in DNS policy")
	}
}

func TestClashV2EmptyNodes(t *testing.T) {
	yaml := ClashV2([]model.Node{})
	if !strings.Contains(yaml, "proxies: []") {
		t.Error("empty config should have empty proxies array")
	}
	if !strings.Contains(yaml, "MATCH,DIRECT") {
		t.Error("empty config should have DIRECT fallback")
	}
}

func TestClashV2SimpleRules(t *testing.T) {
	cfg := ClashConfig{
		Nodes: []model.Node{{
			ID: "ss", Protocol: "shadowsocks", Status: "running",
			Port: 8388, Password: "secret", Host: "1.2.3.4",
		}},
		UseRuleSet: false,
	}

	yaml := cfg.Generate()

	// 不应该有 rule-providers
	if strings.Contains(yaml, "rule-providers:") {
		t.Error("simple mode should not have rule-providers")
	}

	// 应该有基本规则
	basicRules := []string{
		"IP-CIDR,10.0.0.0/8,DIRECT",
		"DOMAIN-SUFFIX,google.com,Proxy",
		"DOMAIN-SUFFIX,baidu.com,China",
		"GEOIP,cn,China",
	}

	for _, r := range basicRules {
		if !strings.Contains(yaml, r) {
			t.Errorf("missing basic rule: %s", r)
		}
	}
}

func TestClashV2ProxyGeneration(t *testing.T) {
	nodes := []model.Node{
		{
			ID: "ss", Protocol: "shadowsocks", Status: "running",
			Port: 8388, Password: "secret", Host: "1.2.3.4",
		},
		{
			ID: "trojan", Protocol: "trojan-tls", Status: "running",
			Port: 443, Password: "pass", Host: "example.com",
		},
		{
			ID: "hy2", Protocol: "hysteria2", Status: "running",
			Port: 8443, Password: "pass", Host: "example.com",
		},
	}

	yaml := ClashV2(nodes)

	// 验证每个协议都生成了代理
	if !strings.Contains(yaml, `type: "ss"`) && !strings.Contains(yaml, "type: ss") {
		t.Error("missing shadowsocks proxy")
	}
	if !strings.Contains(yaml, `type: "trojan"`) && !strings.Contains(yaml, "type: trojan") {
		t.Error("missing trojan proxy")
	}
	if !strings.Contains(yaml, `type: "hysteria2"`) && !strings.Contains(yaml, "type: hysteria2") {
		t.Error("missing hysteria2 proxy")
	}

	// 验证代理名称在代理组中
	if !strings.Contains(yaml, "EasyNode shadowsocks") {
		t.Error("proxy name not found in groups")
	}
}

func TestClashV2StreamingRules(t *testing.T) {
	yaml := ClashV2([]model.Node{{
		ID: "ss", Protocol: "shadowsocks", Status: "running",
		Port: 8388, Password: "secret", Host: "1.2.3.4",
	}})

	streamingSites := []string{
		"netflix.com",
		"youtube.com",
		"spotify.com",
		"twitch.tv",
	}

	for _, site := range streamingSites {
		if !strings.Contains(yaml, site+",Streaming") {
			t.Errorf("missing streaming rule for: %s", site)
		}
	}
}
