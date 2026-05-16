package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"easynode/internal/model"
	"easynode/internal/store"
	"easynode/internal/util"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	password := "testpass123"
	hash, _ := util.HashPassword(password)
	sessionToken := util.Token(24)
	_ = st.Update(func(s *model.AppState) error {
		s.SetupDone = true
		s.AdminPasswordHash = hash
		s.SessionTokenHash = util.SHA256Hex(sessionToken)
		return nil
	})
	srv := &Server{store: st, dataDir: dir, mux: http.NewServeMux()}
	srv.metrics = startMetricsLoop(st)
	srv.routes()
	return srv, password
}

func TestLoginLockoutAfterFiveFailures(t *testing.T) {
	srv, _ := testServer(t)

	loginWith := func(pw string) int {
		body, _ := json.Marshal(map[string]string{"password": pw})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		srv.Login(w, req)
		return w.Code
	}

	for i := 0; i < 5; i++ {
		code := loginWith("wrongpass")
		if code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, code)
		}
	}

	code := loginWith("wrongpass")
	if code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: got %d, want 429", code)
	}

	code = loginWith("testpass123")
	if code != http.StatusTooManyRequests {
		t.Fatalf("correct password during lockout: got %d, want 429", code)
	}
}
