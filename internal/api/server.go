package api

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"easynode/internal/core/chain"
	"easynode/internal/core/detector"
	"easynode/internal/core/recommender"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/model"
	"easynode/internal/store"
	"easynode/internal/util"
)

type Server struct {
	store   *store.Store
	dataDir string
	static  embed.FS
	mux     *http.ServeMux
}

func New(st *store.Store, dataDir string, static embed.FS) *Server {
	s := &Server{store: st, dataDir: dataDir, static: static, mux: http.NewServeMux()}
	_ = s.ensureRunnableNodes()
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/v1/setup/status", s.SetupStatus)
	s.mux.HandleFunc("/api/v1/setup", s.Setup)
	s.mux.HandleFunc("/api/v1/login", s.Login)
	s.mux.HandleFunc("/api/v1/logout", s.Logout)
	s.mux.HandleFunc("/api/v1/state", s.auth(s.State))
	s.mux.HandleFunc("/api/v1/settings", s.auth(s.UpdateSettings))
	s.mux.HandleFunc("/api/v1/nodes", s.auth(s.Nodes))
	s.mux.HandleFunc("/api/v1/nodes/", s.auth(s.NodeAction))
	s.mux.HandleFunc("/api/v1/subscribe/", s.Subscribe)
	s.mux.HandleFunc("/api/v1/chain/generate-code", s.auth(s.GenerateCode))
	s.mux.HandleFunc("/api/v1/chain/pair", s.auth(s.Pair))
	s.mux.HandleFunc("/api/v1/sing-box/config", s.auth(s.SingBoxConfig))
	s.mux.HandleFunc("/", s.Static)
}

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
	env := detector.Detect(req.Domain, req.IPDirect)
	recs := recommender.Recommend(env)
	nodes := nodesFromRecommendations(recs, req.Protocols, req.Domain, req.IPDirect)
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
		st.SessionToken = sessionToken
		st.LoginFailures = 0
		st.LockoutUntil = time.Time{}
		if strings.HasPrefix(req.PanelPath, "/") && len(req.PanelPath) > 1 {
			st.PanelPath = req.PanelPath
		}
		st.Domain = req.Domain
		st.IPDirect = req.IPDirect
		st.Nodes = nodes
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers)
	_ = singbox.RestartService()
	setSession(w, st.SessionToken)
	writeJSON(w, http.StatusOK, publicState(st))
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
		st.SessionToken = sessionToken
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
		st.SessionToken = util.Token(24)
		return nil
	})
	http.SetCookie(w, &http.Cookie{Name: "easynode_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) State(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, publicState(s.store.Snapshot()))
}

func (s *Server) Nodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Nodes)
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
	err := s.store.Update(func(st *model.AppState) error {
		if newHash != "" {
			st.AdminPasswordHash = newHash
			st.SessionToken = util.Token(24)
		}
		if strings.HasPrefix(req.PanelPath, "/") && len(req.PanelPath) > 1 {
			st.PanelPath = req.PanelPath
		}
		st.Domain = req.Domain
		st.IPDirect = req.IPDirect
		host := req.Domain
		if req.IPDirect || host == "" {
			host = "127.0.0.1"
		}
		for i := range st.Nodes {
			st.Nodes[i].Host = host
			st.Nodes[i].SubscribeLink = subscribe.Link(st.Nodes[i])
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	next := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, next.Nodes, next.ChainPeers)
	_ = singbox.RestartService()
	if newHash != "" {
		setSession(w, next.SessionToken)
	}
	writeJSON(w, http.StatusOK, publicState(next))
}

func (s *Server) NodeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/"), "/")
	if len(parts) != 2 || parts[1] != "toggle" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	err := s.store.Update(func(st *model.AppState) error {
		for i := range st.Nodes {
			if st.Nodes[i].ID == id {
				if st.Nodes[i].Status == "running" {
					st.Nodes[i].Status = "stopped"
				} else {
					st.Nodes[i].Status = "running"
				}
				return nil
			}
		}
		return errors.New("node not found")
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, st.Nodes)
}

func (s *Server) Subscribe(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/subscribe/")
	st := s.store.Snapshot()
	if key != st.SubscribeKey {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(subscribe.V2rayN(st.Nodes)))
}

func (s *Server) GenerateCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	code := chain.NewCode(req.Endpoint)
	err := s.store.Update(func(st *model.AppState) error {
		st.PairingCodes = append(st.PairingCodes, code)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, code)
}

func (s *Server) Pair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Code        string `json:"code"`
		Endpoint    string `json:"my_endpoint"`
		PublicKey   string `json:"my_public_key"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var paired model.PairingCode
	err := s.store.Update(func(st *model.AppState) error {
		c, err := chain.Pair(st.PairingCodes, req.Code, req.Endpoint, req.PublicKey)
		if err != nil {
			return err
		}
		paired = c
		for i := range st.PairingCodes {
			if st.PairingCodes[i].Code == req.Code {
				st.PairingCodes[i].Used = true
			}
		}
		name := req.DisplayName
		if name == "" {
			name = "Exit " + req.Code
		}
		st.ChainPeers = append(st.ChainPeers, model.ChainPeer{ID: util.Token(6), Name: name, Endpoint: c.Endpoint, PublicKey: c.PublicKey, Status: "paired", CreatedAt: time.Now()})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer_public_key": paired.PublicKey, "peer_endpoint": paired.Endpoint, "tunnel_config": paired})
}

func (s *Server) SingBoxConfig(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	path, err := singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) Static(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	if st.SetupDone && st.PanelPath != "" && r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, st.PanelPath) && !strings.HasPrefix(r.URL.Path, "/assets/") {
		http.NotFound(w, r)
		return
	}
	sub, err := fs.Sub(s.static, "dist")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, st.PanelPath)
	if path == "" || path == "/" {
		path = "index.html"
	} else {
		path = strings.TrimPrefix(path, "/")
	}
	if _, err := sub.Open(path); err != nil {
		path = "index.html"
	}
	b, err := fs.ReadFile(sub, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if typ := mime.TypeByExtension(filepath.Ext(path)); typ != "" {
		w.Header().Set("Content-Type", typ)
	}
	http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(b))
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("easynode_session"); err == nil {
			st := s.store.Snapshot()
			if st.SessionToken != "" && c.Value == st.SessionToken {
				next(w, r)
				return
			}
		}
		writeError(w, http.StatusUnauthorized, errors.New("login required"))
	}
}

func nodesFromRecommendations(recs []model.Recommendation, selected []string, domain string, ipDirect bool) []model.Node {
	want := map[string]bool{}
	for _, p := range selected {
		want[p] = true
	}
	host := domain
	if ipDirect || host == "" {
		host = "127.0.0.1"
	}
	ports := map[string]int{"vless-reality": 443, "hysteria2": 8443, "trojan-tls": 2053, "vless-ws-tls": 2083, "tuic": 9443}
	nodes := []model.Node{}
	for _, rec := range recs {
		enabled := rec.Enabled
		if len(want) > 0 {
			enabled = want[rec.Protocol]
		}
		if !enabled {
			continue
		}
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
		} else {
			n.Status = "stopped"
		}
		lat := 18 + len(nodes)*11
		n.LatencyMS = &lat
		if n.Status == "running" {
			n.SubscribeLink = subscribe.Link(n)
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func (s *Server) ensureRunnableNodes() error {
	changed := false
	err := s.store.Update(func(st *model.AppState) error {
		host := st.Domain
		if st.IPDirect || host == "" {
			host = "127.0.0.1"
		}
		for i := range st.Nodes {
			n := &st.Nodes[i]
			n.Host = host
			if n.Protocol == "vless-reality" {
				if n.RealityPrivateKey == "" || n.RealityPublicKey == "" || n.RealityShortID == "" {
					m := singbox.GenerateRealityMaterial()
					n.RealityPrivateKey = m.PrivateKey
					n.RealityPublicKey = m.PublicKey
					n.RealityShortID = m.ShortID
					changed = true
				}
				n.Status = "running"
				n.SubscribeLink = subscribe.Link(*n)
				continue
			}
			if n.Status == "running" {
				n.Status = "stopped"
				changed = true
			}
			n.SubscribeLink = ""
		}
		return nil
	})
	if err != nil {
		return err
	}
	if changed {
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers)
		_ = singbox.RestartService()
	}
	return nil
}

func setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: "easynode_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(30 * 24 * time.Hour)})
}

func publicState(st model.AppState) model.AppState {
	st.AdminPassword = ""
	st.AdminPasswordHash = ""
	st.SessionToken = ""
	st.LoginFailures = 0
	st.LockoutUntil = time.Time{}
	st.PairingCodes = nil
	return st
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
