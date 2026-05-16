package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"easynode/internal/core/chain"
	"easynode/internal/model"
	"easynode/internal/util"
)

func TestChainPairWithBundle(t *testing.T) {
	srv, _ := testServer(t)

	_ = srv.store.Update(func(st *model.AppState) error {
		st.Nodes = []model.Node{{
			ID: "vless-reality", Protocol: "vless-reality", Status: "running",
			Port: 443, UUID: util.UUID(), Host: "1.2.3.4",
			RealityPrivateKey: "priv", RealityPublicKey: "pub", RealityShortID: "abcd",
		}}
		return nil
	})

	body, _ := json.Marshal(map[string]string{"endpoint": "https://1.2.3.4:8443"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chain/generate-code", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "easynode_session", Value: validSession(t, srv)})
	w := httptest.NewRecorder()
	srv.GenerateCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generate-code: got %d, body: %s", w.Code, w.Body.String())
	}

	var codeResp model.PairingCode
	if err := json.NewDecoder(w.Body).Decode(&codeResp); err != nil {
		t.Fatal(err)
	}
	if codeResp.Bundle == "" {
		t.Fatal("expected bundle in response")
	}

	bundle, err := chain.DecodeBundle(codeResp.Bundle)
	if err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Endpoint != "https://1.2.3.4:8443" {
		t.Fatalf("bundle endpoint = %q", bundle.Endpoint)
	}
	if bundle.OutboundLink == "" {
		t.Fatal("bundle missing outbound link")
	}
}

func TestChainPairingDisabled(t *testing.T) {
	srv, _ := testServer(t)

	_ = srv.store.Update(func(st *model.AppState) error {
		st.ChainPairingDisabled = true
		st.Nodes = []model.Node{{
			ID: "vless-reality", Protocol: "vless-reality", Status: "running",
			Port: 443, UUID: util.UUID(), Host: "1.2.3.4",
			RealityPrivateKey: "priv", RealityPublicKey: "pub", RealityShortID: "abcd",
		}}
		return nil
	})

	body, _ := json.Marshal(map[string]string{"endpoint": "https://1.2.3.4:8443"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chain/generate-code", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "easynode_session", Value: validSession(t, srv)})
	w := httptest.NewRecorder()
	srv.GenerateCode(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when pairing disabled, got %d", w.Code)
	}
}

func validSession(t *testing.T, srv *Server) string {
	t.Helper()
	token := util.Token(24)
	_ = srv.store.Update(func(st *model.AppState) error {
		st.SessionTokenHash = util.SHA256Hex(token)
		return nil
	})
	return token
}
