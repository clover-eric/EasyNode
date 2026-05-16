package api

import (
	"easynode/internal/model"
	"net/http"
	"strings"
	"testing"
)

func TestConfiguredHostUsesPublicRequestHostForIPDirect(t *testing.T) {
	r := &http.Request{Host: "203.0.113.10:8088"}
	got := configuredHost("", true, r)
	if got != "203.0.113.10" {
		t.Fatalf("configuredHost() = %q, want request host", got)
	}
}

func TestConfiguredHostRejectsLocalRequestHost(t *testing.T) {
	r := &http.Request{Host: "127.0.0.1:8088"}
	got := publicHostFromRequest(r)
	if got != "" {
		t.Fatalf("publicHostFromRequest() = %q, want empty local host", got)
	}
}

func TestNodesForRequestRewritesIPDirectLoopback(t *testing.T) {
	st := model.AppState{
		IPDirect: true,
		Nodes: []model.Node{{
			Protocol:         "vless-reality",
			Status:           "running",
			Host:             "127.0.0.1",
			Port:             443,
			UUID:             "11111111-1111-1111-1111-111111111111",
			RealityPublicKey: "pub",
			RealityShortID:   "sid",
		}},
	}
	nodes := nodesForRequest(st, &http.Request{Host: "203.0.113.10:8088"})
	if nodes[0].Host != "203.0.113.10" {
		t.Fatalf("Host = %q, want request host", nodes[0].Host)
	}
	if !strings.Contains(nodes[0].SubscribeLink, "@203.0.113.10:443") {
		t.Fatalf("SubscribeLink was not rewritten: %s", nodes[0].SubscribeLink)
	}
}

func TestShadowsocksRunnableWithoutCert(t *testing.T) {
	if !protocolRunnable("shadowsocks", false) {
		t.Fatal("shadowsocks should run without certificate")
	}
	n := newNodeFromRecommendation(model.Recommendation{Protocol: "shadowsocks"}, "203.0.113.10", false)
	if n.Status != "running" || n.Port != 8388 || n.Password == "" {
		t.Fatalf("unexpected shadowsocks node: %#v", n)
	}
}
