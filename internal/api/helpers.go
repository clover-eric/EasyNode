package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"easynode/internal/core/detector"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/model"
	"easynode/internal/util"
)

func nodesFromRecommendations(recs []model.Recommendation, selected []string, host string, certReady bool) []model.Node {
	want := map[string]bool{}
	for _, p := range selected {
		want[p] = true
	}
	if host == "" {
		host = "127.0.0.1"
	}
	nodes := []model.Node{}
	for _, rec := range recs {
		enabled := rec.Enabled
		if len(want) > 0 {
			enabled = want[rec.Protocol]
		}
		if !enabled {
			continue
		}
		n := newNodeFromRecommendation(rec, host, certReady)
		lat := 18 + len(nodes)*11
		n.LatencyMS = &lat
		nodes = append(nodes, n)
	}
	return nodes
}

func newNodeFromRecommendation(rec model.Recommendation, host string, certReady bool) model.Node {
	ports := map[string]int{"vless-reality": 443, "shadowsocks": 8388, "hysteria2": 8443, "trojan-tls": 2053, "vless-ws-tls": 2083, "tuic": 9443, "clash": 0}
	n := model.Node{
		ID: rec.Protocol, Protocol: rec.Protocol, Transport: rec.Transport, Security: rec.Security,
		Label: rec.Label, Description: rec.Description, Priority: rec.Priority, Status: "running",
		Port: ports[rec.Protocol], UUID: util.UUID(), Password: util.Token(12), Host: host, CreatedAt: time.Now(),
	}
	if n.Protocol == "vless-reality" {
		m := singbox.GenerateRealityMaterial()
		n.RealityPrivateKey = m.PrivateKey
		n.RealityPublicKey = m.PublicKey
		n.RealityShortID = m.ShortID
	}
	if n.Protocol == "vless-reality" && (n.RealityPrivateKey == "" || n.RealityPublicKey == "") {
		n.Status = "stopped"
	}
	if !protocolRunnable(n.Protocol, certReady) {
		n.Status = "stopped"
	}
	if n.Status == "running" {
		n.SubscribeLink = subscribe.Link(n)
	}
	return n
}

func protocolRunnable(protocol string, certReady bool) bool {
	if protocol == "vless-reality" || protocol == "shadowsocks" || protocol == "clash" {
		return true
	}
	switch protocol {
	case "trojan-tls", "hysteria2", "vless-ws-tls", "tuic":
		return certReady
	default:
		return false
	}
}

func protocolUnavailableReason(protocol string) string {
	switch protocol {
	case "trojan-tls", "hysteria2", "vless-ws-tls", "tuic":
		return "this protocol requires certificate automation before it can be enabled"
	default:
		return "this protocol is not available yet"
	}
}

func configuredHost(domain string, ipDirect bool, r *http.Request) string {
	if !ipDirect {
		return domain
	}
	if domain != "" && !isLocalhost(domain) {
		return domain
	}
	if h := publicHostFromRequest(r); h != "" {
		return h
	}
	if env := detector.Detect("", true); env.PublicIP != "" {
		return env.PublicIP
	}
	return "127.0.0.1"
}

func publicHostFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if isLocalhost(host) {
		return ""
	}
	return host
}

func nodesForRequest(st model.AppState, r *http.Request) []model.Node {
	nodes := append([]model.Node(nil), st.Nodes...)
	host := configuredHost(st.Domain, st.IPDirect, r)
	if host == "" {
		return nodes
	}
	for i := range nodes {
		nodes[i].Host = host
		if nodes[i].Status == "running" {
			nodes[i].SubscribeLink = subscribe.Link(nodes[i])
		}
	}
	return nodes
}

func isLocalhost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || h == "localhost" || h == "::1" || h == "0.0.0.0" || strings.HasPrefix(h, "127.") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()
	}
	return false
}

func hasProtocol(nodes []model.Node, protocol string) bool {
	for _, n := range nodes {
		if n.Protocol == protocol {
			return true
		}
	}
	return false
}

func setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: "easynode_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(30 * 24 * time.Hour)})
}

func publicState(st model.AppState) model.AppState {
	st.AdminPassword = ""
	st.AdminPasswordHash = ""
	st.SessionTokenHash = ""
	st.LoginFailures = 0
	st.LockoutUntil = time.Time{}
	st.PairingCodes = nil
	return st
}

func withPanelURL(st model.AppState) map[string]any {
	b, _ := json.Marshal(st)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if !st.IPDirect && st.Domain != "" && st.CertReady {
		out["panel_url"] = "https://" + st.Domain + ":8443" + st.PanelPath
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error(), "status": strconv.Itoa(status)})
}

func method(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}
