package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"easynode/internal/core/cert"
	"easynode/internal/core/detector"
	"easynode/internal/core/recommender"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/model"
	"easynode/internal/util"
)

func (s *Server) SetupStatus(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"setup_done": st.SetupDone, "panel_path": st.PanelPath})
}

func (s *Server) Setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Password  string   `json:"password"`
		PanelPath string   `json:"panel_path"`
		Domain    string   `json:"domain"`
		IPDirect  bool     `json:"ip_direct"`
		Protocols []string `json:"protocols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, errors.New("password must be at least 8 characters"))
		return
	}
	if !req.IPDirect && req.Domain == "" {
		writeError(w, http.StatusBadRequest, errors.New("domain required unless IP direct mode enabled"))
		return
	}
	certReady := false
	certPath := ""
	keyPath := ""
	if !req.IPDirect {
		c, err := cert.Ensure(req.Domain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		certReady = c.Ready
		certPath = c.CertPath
		keyPath = c.KeyPath
	}
	env := detector.Detect(req.Domain, req.IPDirect)
	env.TLSReady = certReady
	recs := recommender.Recommend(env)
	host := configuredHost(req.Domain, req.IPDirect, r)
	nodes := nodesFromRecommendations(recs, req.Protocols, host, certReady)
	hash, err := util.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	sessionToken := util.Token(24)
	if err := s.store.Update(func(st *model.AppState) error {
		st.SetupDone = true
		st.AdminPasswordHash = hash
		st.AdminPassword = ""
		st.SessionTokenHash = util.SHA256Hex(sessionToken)
		st.LoginFailures = 0
		st.LockoutUntil = time.Time{}
		if strings.HasPrefix(req.PanelPath, "/") && len(req.PanelPath) > 1 {
			st.PanelPath = req.PanelPath
		}
		st.Domain = host
		st.IPDirect = req.IPDirect
		st.CertReady = certReady
		st.CertPath = certPath
		st.KeyPath = keyPath
		st.Nodes = nodes
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	setSession(w, sessionToken)
	resp := publicState(st)
	writeJSON(w, http.StatusOK, withPanelURL(resp))
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	st := s.store.Snapshot()
	if !st.SetupDone {
		writeError(w, http.StatusUnauthorized, errors.New("invalid password"))
		return
	}
	if st.LockoutUntil.After(time.Now()) {
		writeError(w, http.StatusTooManyRequests, errors.New("too many login attempts, try later"))
		return
	}
	if !util.VerifyPassword(req.Password, st.AdminPasswordHash) {
		_ = s.store.Update(func(st *model.AppState) error {
			st.LoginFailures++
			if st.LoginFailures >= 5 {
				st.LockoutUntil = time.Now().Add(5 * time.Minute)
			}
			return nil
		})
		writeError(w, http.StatusUnauthorized, errors.New("invalid password"))
		return
	}
	sessionToken := util.Token(24)
	if err := s.store.Update(func(st *model.AppState) error {
		st.SessionTokenHash = util.SHA256Hex(sessionToken)
		st.LoginFailures = 0
		st.LockoutUntil = time.Time{}
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	setSession(w, sessionToken)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	_ = s.store.Update(func(st *model.AppState) error {
		st.SessionTokenHash = util.SHA256Hex(util.Token(24))
		return nil
	})
	http.SetCookie(w, &http.Cookie{Name: "easynode_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) State(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, publicState(s.enrichedState()))
}

func (s *Server) IPPurity(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, detector.CheckPurity())
}

func (s *Server) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		PanelPath       string `json:"panel_path"`
		Domain          string `json:"domain"`
		IPDirect        bool   `json:"ip_direct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	if !util.VerifyPassword(req.CurrentPassword, st.AdminPasswordHash) {
		writeError(w, http.StatusUnauthorized, errors.New("current password invalid"))
		return
	}
	if req.NewPassword != "" && len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, errors.New("new password must be at least 8 characters"))
		return
	}
	if !req.IPDirect && req.Domain == "" {
		writeError(w, http.StatusBadRequest, errors.New("domain required unless IP direct mode enabled"))
		return
	}
	var newHash string
	if req.NewPassword != "" {
		hash, err := util.HashPassword(req.NewPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		newHash = hash
	}
	certReady := false
	certPath := ""
	keyPath := ""
	if !req.IPDirect {
		c, err := cert.Ensure(req.Domain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		certReady = c.Ready
		certPath = c.CertPath
		keyPath = c.KeyPath
	}
	var newSessionToken string
	err := s.store.Update(func(st *model.AppState) error {
		if newHash != "" {
			st.AdminPasswordHash = newHash
			newSessionToken = util.Token(24)
			st.SessionTokenHash = util.SHA256Hex(newSessionToken)
		}
		if strings.HasPrefix(req.PanelPath, "/") && len(req.PanelPath) > 1 {
			st.PanelPath = req.PanelPath
		}
		host := configuredHost(req.Domain, req.IPDirect, r)
		st.Domain = host
		st.IPDirect = req.IPDirect
		st.CertReady = certReady
		st.CertPath = certPath
		st.KeyPath = keyPath
		for i := range st.Nodes {
			st.Nodes[i].Host = host
			if protocolRunnable(st.Nodes[i].Protocol, st.CertReady) && st.Nodes[i].Status == "running" {
				st.Nodes[i].SubscribeLink = subscribe.Link(st.Nodes[i])
			} else {
				st.Nodes[i].SubscribeLink = ""
			}
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	next := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, next.Nodes, next.ChainPeers, next.CertPath, next.KeyPath)
	_ = singbox.RestartService()
	if newSessionToken != "" {
		setSession(w, newSessionToken)
	}
	writeJSON(w, http.StatusOK, publicState(next))
}
