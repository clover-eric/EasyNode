package recommender

import (
	"testing"

	"easynode/internal/model"
)

func TestRecommendDefaults(t *testing.T) {
	recs := Recommend(model.Environment{HasIPv4: true, UDPAvailable: true, TLSReady: true})
	want := map[string]bool{"vless-reality": true, "shadowsocks": true, "hysteria2": true, "trojan-tls": true}
	for _, rec := range recs {
		if rec.Enabled {
			want[rec.Protocol] = false
		}
	}
	for proto, missing := range want {
		if missing {
			t.Fatalf("%s should be enabled", proto)
		}
	}
}
