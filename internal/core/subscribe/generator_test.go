package subscribe

import (
	"strings"
	"testing"

	"easynode/internal/model"
)

func TestClashIncludesRoutingAndProxyGroups(t *testing.T) {
	nodes := []model.Node{
		{
			Protocol:         "vless-reality",
			Status:           "running",
			Host:             "example.com",
			Port:             443,
			UUID:             "11111111-1111-1111-1111-111111111111",
			RealityPublicKey: "pub",
			RealityShortID:   "abcd",
		},
		{
			Protocol: "shadowsocks",
			Status:   "running",
			Host:     "example.com",
			Port:     8388,
			Password: "secret",
		},
		{
			Protocol: "hysteria2",
			Status:   "stopped",
			Host:     "example.com",
			Port:     8443,
			Password: "secret",
		},
	}
	yaml := ClashV2(nodes)
	want := []string{
		`mixed-port: 7890`,
		`unified-delay: true`,
		`tcp-concurrent: true`,
		`enhanced-mode: fake-ip`,
		`name: "GLOBAL"`,
		`name: "Proxy"`,
		`name: "Auto"`,
		`name: "Fallback"`,
		`name: "LoadBalance"`,
		`type: url-test`,
		`type: fallback`,
		`type: load-balance`,
		`RULE-SET,proxy,Proxy`,
		`RULE-SET,direct,China`,
		`name: "EasyNode shadowsocks"`,
		`type: "ss"`,
		`cipher: "chacha20-ietf-poly1305"`,
		`name: "EasyNode vless-reality"`,
		`type: "vless"`,
		`skip-cert-verify: false`,
		`reality-opts:`,
	}
	for _, s := range want {
		if !strings.Contains(yaml, s) {
			t.Fatalf("clash yaml missing %q\n%s", s, yaml)
		}
	}
	if strings.Contains(yaml, "hysteria2") {
		t.Fatalf("stopped node leaked into clash yaml:\n%s", yaml)
	}
}

func TestShadowsocksLink(t *testing.T) {
	link := Link(model.Node{
		Protocol: "shadowsocks",
		Host:     "example.com",
		Port:     8388,
		Password: "secret",
	})
	if !strings.HasPrefix(link, "ss://") || !strings.Contains(link, "@example.com:8388") {
		t.Fatalf("unexpected shadowsocks link: %s", link)
	}
}

func TestClashSkipsInvalidRealityNode(t *testing.T) {
	yaml := ClashV2([]model.Node{{
		Protocol: "vless-reality",
		Status:   "running",
		Host:     "example.com",
		Port:     443,
		UUID:     "11111111-1111-1111-1111-111111111111",
	}})
	if strings.Contains(yaml, "EasyNode vless-reality") {
		t.Fatalf("invalid reality node leaked into clash yaml:\n%s", yaml)
	}
	if !strings.Contains(yaml, "MATCH,DIRECT") {
		t.Fatalf("empty clash yaml should fall back to direct:\n%s", yaml)
	}
}
