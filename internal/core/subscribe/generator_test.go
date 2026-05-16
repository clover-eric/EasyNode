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
			Protocol: "hysteria2",
			Status:   "stopped",
			Host:     "example.com",
			Port:     8443,
			Password: "secret",
		},
	}
	yaml := Clash(nodes)
	want := []string{
		`mixed-port: 7890`,
		`unified-delay: true`,
		`tcp-concurrent: true`,
		`enhanced-mode: fake-ip`,
		`type: url-test`,
		`type: fallback`,
		`GEOSITE,geolocation-!cn,🌍 国外流量`,
		`GEOIP,cn,🇨🇳 国内直连,no-resolve`,
		`name: "EasyNode vless-reality"`,
		`type: "vless"`,
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
