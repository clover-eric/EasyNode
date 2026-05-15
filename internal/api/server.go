package api

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"easynode/internal/core/cert"
	"easynode/internal/core/chain"
	"easynode/internal/core/detector"
	"easynode/internal/core/recommender"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/core/traffic"
	"easynode/internal/model"
	"easynode/internal/store"
	"easynode/internal/util"

	qrcode "github.com/skip2/go-qrcode"
)

type Server struct {
	store   *store.Store
	dataDir string
	static  embed.FS
	mux     *http.ServeMux
	upgrade upgradeState
	build   BuildInfo
}

type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

type upgradeState struct {
	mu        sync.RWMutex
	Running   bool      `json:"running"`
	Progress  int       `json:"progress"`
	Step      string    `json:"step"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	Backup    string    `json:"backup,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func New(st *store.Store, dataDir string, static embed.FS, build BuildInfo) *Server {
	s := &Server{store: st, dataDir: dataDir, static: static, mux: http.NewServeMux(), build: build}
	_ = s.ensureRunnableNodes()
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) StateSnapshot() model.AppState {
	return s.store.Snapshot()
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/v1/setup/status", s.SetupStatus)
	s.mux.HandleFunc("/api/v1/setup", s.Setup)
	s.mux.HandleFunc("/api/v1/login", s.Login)
	s.mux.HandleFunc("/api/v1/logout", s.Logout)
	s.mux.HandleFunc("/api/v1/state", s.auth(s.State))
	s.mux.HandleFunc("/api/v1/ip/purity", s.auth(s.IPPurity))
	s.mux.HandleFunc("/api/v1/settings", s.auth(s.UpdateSettings))
	s.mux.HandleFunc("/api/v1/system/upgrade", s.auth(s.Upgrade))
	s.mux.HandleFunc("/api/v1/system/upgrade/status", s.auth(s.UpgradeStatus))
	s.mux.HandleFunc("/api/v1/system/update-info", s.auth(s.UpdateInfo))
	s.mux.HandleFunc("/api/v1/qrcode/subscribe", s.auth(s.SubscribeQRCode))
	s.mux.HandleFunc("/api/v1/qrcode/node/", s.auth(s.NodeQRCode))
	s.mux.HandleFunc("/api/v1/chain/public/status", s.ChainPublicStatus)
	s.mux.HandleFunc("/api/v1/chain/public/paired", s.ChainPublicPaired)
	s.mux.HandleFunc("/api/v1/chain/public/unpaired", s.ChainPublicUnpaired)
	s.mux.HandleFunc("/api/v1/nodes", s.auth(s.Nodes))
	s.mux.HandleFunc("/api/v1/nodes/add", s.auth(s.AddNode))
	s.mux.HandleFunc("/api/v1/nodes/remove", s.auth(s.RemoveNode))
	s.mux.HandleFunc("/api/v1/nodes/", s.auth(s.NodeAction))
	s.mux.HandleFunc("/api/v1/subscribe/", s.Subscribe)
	s.mux.HandleFunc("/api/v1/chain/generate-code", s.auth(s.GenerateCode))
	s.mux.HandleFunc("/api/v1/chain/pair", s.auth(s.Pair))
	s.mux.HandleFunc("/api/v1/chain/remove", s.auth(s.RemoveChainPeer))
	s.mux.HandleFunc("/api/v1/chain/client/remove", s.auth(s.RemoveChainClient))
	s.mux.HandleFunc("/api/v1/chain/accepting", s.auth(s.SetChainAccepting))
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
	nodes := nodesFromRecommendations(recs, req.Protocols, req.Domain, req.IPDirect, certReady)
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
	setSession(w, st.SessionToken)
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
	writeJSON(w, http.StatusOK, publicState(s.enrichedState()))
}

func (s *Server) ChainPublicStatus(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"pairing_disabled": st.ChainPairingDisabled,
		"panel_path":       st.PanelPath,
		"domain":           st.Domain,
		"updated_at":       st.UpdatedAt,
	})
}

func (s *Server) ChainPublicPaired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Code         string `json:"code"`
		Endpoint     string `json:"endpoint"`
		PublicKey    string `json:"public_key"`
		OutboundLink string `json:"outbound_link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		if st.ChainPairingDisabled {
			return errors.New("chain pairing is disabled on this server")
		}
		found := false
		for i := range st.PairingCodes {
			if st.PairingCodes[i].Code == req.Code && st.PairingCodes[i].ExpiresAt.After(time.Now()) {
				st.PairingCodes[i].Used = true
				found = true
				break
			}
		}
		if !found && req.OutboundLink != "" {
			for _, n := range st.Nodes {
				if n.Status == "running" && subscribe.Link(n) == req.OutboundLink {
					found = true
					break
				}
			}
		}
		if !found {
			return errors.New("pairing code invalid or expired")
		}
		upsertChainClient(st, model.ChainClient{ID: util.Token(6), Name: chainClientName(req.Endpoint), Endpoint: req.Endpoint, PublicKey: req.PublicKey, OutboundLink: req.OutboundLink, Status: "paired", CreatedAt: time.Now()})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) ChainPublicUnpaired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ExitEndpoint string `json:"exit_endpoint"`
		Endpoint     string `json:"endpoint"`
		PublicKey    string `json:"public_key"`
		OutboundLink string `json:"outbound_link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	removed := false
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainPeers[:0]
		for _, peer := range st.ChainPeers {
			if chainPeerMatches(peer, req.ExitEndpoint, req.PublicKey, req.OutboundLink) {
				removed = true
				continue
			}
			next = append(next, peer)
		}
		st.ChainPeers = next
		nextClients := st.ChainClients[:0]
		for _, client := range st.ChainClients {
			if chainClientMatches(client, req.Endpoint, req.PublicKey) {
				removed = true
				continue
			}
			nextClients = append(nextClients, client)
		}
		st.ChainClients = nextClients
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if removed {
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
		_ = singbox.RestartService()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "removed": removed})
}

func (s *Server) IPPurity(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, detector.CheckPurity())
}

func (s *Server) enrichedState() model.AppState {
	st := s.store.Snapshot()
	bytesByPort := traffic.PortBytes()
	mainlandLatency := detector.MainlandLatency()
	for i := range st.Nodes {
		st.Nodes[i].TrafficUsed = bytesByPort[st.Nodes[i].Port]
		if st.Nodes[i].Status == "running" {
			st.Nodes[i].LatencyMS = mainlandLatency
		} else {
			st.Nodes[i].LatencyMS = nil
		}
	}
	for i := range st.ChainPeers {
		disabled, checked, latency := remotePairingDisabled(st.ChainPeers[i].Endpoint)
		st.ChainPeers[i].RemotePairingDisabled = disabled
		st.ChainPeers[i].RemoteStatusCheckedAt = checked
		st.ChainPeers[i].RemoteLatencyMS = latency
	}
	for i := range st.ChainClients {
		st.ChainClients[i].RemoteLatencyMS = detector.EndpointLatency(st.ChainClients[i].Endpoint)
	}
	return st
}

func remotePairingDisabled(endpoint string) (bool, time.Time, *int) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, time.Time{}, nil
	}
	u.Path = "/api/v1/chain/public/status"
	u.RawQuery = ""
	client := http.Client{Timeout: 2 * time.Second}
	start := time.Now()
	resp, err := client.Get(u.String())
	if err != nil {
		return false, time.Time{}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, time.Time{}, nil
	}
	var out struct {
		PairingDisabled bool `json:"pairing_disabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, time.Time{}, nil
	}
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return out.PairingDisabled, time.Now(), &ms
}

func (s *Server) Nodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Nodes)
}

func (s *Server) AddNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	recs := recommender.Recommend(model.Environment{HasIPv4: true, UDPAvailable: true, TLSReady: true})
	var rec *model.Recommendation
	for i := range recs {
		if recs[i].Protocol == req.Protocol {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		writeError(w, http.StatusBadRequest, errors.New("unknown protocol"))
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		for _, n := range st.Nodes {
			if n.Protocol == req.Protocol {
				return errors.New("protocol already added")
			}
		}
		host := st.Domain
		if st.IPDirect || host == "" {
			host = "127.0.0.1"
		}
		node := newNodeFromRecommendation(*rec, host, st.CertReady)
		st.Nodes = append(st.Nodes, node)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, publicState(st))
}

func (s *Server) RemoveNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		next := st.Nodes[:0]
		removed := false
		for _, n := range st.Nodes {
			if n.Protocol == req.Protocol {
				removed = true
				continue
			}
			next = append(next, n)
		}
		if !removed {
			return errors.New("protocol not found")
		}
		st.Nodes = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, publicState(st))
}

func (s *Server) Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	s.upgrade.mu.Lock()
	if s.upgrade.Running {
		s.upgrade.mu.Unlock()
		writeJSON(w, http.StatusAccepted, s.upgradeSnapshot())
		return
	}
	s.upgrade.mu.Unlock()

	info := s.checkUpdateInfo()
	if info.Error == "" && info.LatestCommit != "" && !info.UpdateAvailable {
		writeError(w, http.StatusConflict, errors.New("already latest version"))
		return
	}

	s.upgrade.mu.Lock()
	if s.upgrade.Running {
		s.upgrade.mu.Unlock()
		writeJSON(w, http.StatusAccepted, s.upgradeSnapshot())
		return
	}
	s.upgrade.Running = true
	s.upgrade.Progress = 5
	s.upgrade.Step = "preparing backup"
	s.upgrade.Output = ""
	s.upgrade.Error = ""
	s.upgrade.UpdatedAt = time.Now()
	s.upgrade.mu.Unlock()

	backup := filepath.Join(s.dataDir, "backup-"+time.Now().Format("20060102-150405"))
	go s.runUpgrade(backup)
	writeJSON(w, http.StatusAccepted, s.upgradeSnapshot())
}

func (s *Server) UpgradeStatus(w http.ResponseWriter, r *http.Request) {
	st := s.upgradeSnapshot()
	sys := systemdUpgradeStatus()
	if sys.Output != "" || sys.Running || sys.Progress == 100 {
		writeJSON(w, http.StatusOK, sys)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type updateInfo struct {
	CurrentCommit   string       `json:"current_commit"`
	LatestCommit    string       `json:"latest_commit,omitempty"`
	UpdateAvailable bool         `json:"update_available"`
	Notes           []updateNote `json:"notes,omitempty"`
	Error           string       `json:"error,omitempty"`
}

type updateNote struct {
	Commit  string `json:"commit"`
	Message string `json:"message"`
	Date    string `json:"date,omitempty"`
}

func (s *Server) UpdateInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.checkUpdateInfo())
}

func (s *Server) checkUpdateInfo() updateInfo {
	info := updateInfo{CurrentCommit: s.build.Commit}
	commits, err := fetchGitHubCommits()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	if len(commits) > 0 {
		info.LatestCommit = commits[0].Commit
		info.UpdateAvailable = s.build.Commit == "" || s.build.Commit == "dev" || !strings.HasPrefix(commits[0].Commit, s.build.Commit) && !strings.HasPrefix(s.build.Commit, commits[0].Commit)
		for _, c := range commits {
			if s.build.Commit != "" && s.build.Commit != "dev" && (strings.HasPrefix(c.Commit, s.build.Commit) || strings.HasPrefix(s.build.Commit, c.Commit)) {
				break
			}
			info.Notes = append(info.Notes, c)
			if len(info.Notes) >= 5 {
				break
			}
		}
	}
	return info
}

func fetchGitHubCommits() ([]updateNote, error) {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/clover-eric/EasyNode/commits?sha=main&per_page=5")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("cannot check GitHub updates")
	}
	var data []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	notes := make([]updateNote, 0, len(data))
	for _, c := range data {
		msg := strings.Split(strings.TrimSpace(c.Commit.Message), "\n")[0]
		sha := c.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		notes = append(notes, updateNote{Commit: sha, Message: msg, Date: c.Commit.Author.Date})
	}
	return notes, nil
}

func (s *Server) runUpgrade(backup string) {
	s.setUpgrade(15, "backing up configuration", "", "", backup, true)
	_ = os.MkdirAll(backup, 0700)
	if b, err := os.ReadFile(filepath.Join(s.dataDir, "state.json")); err == nil {
		_ = os.WriteFile(filepath.Join(backup, "state.json"), b, 0600)
	}
	s.setUpgrade(35, "downloading installer", "", "", backup, true)
	_ = exec.Command("systemctl", "reset-failed", "easynode-upgrade").Run()
	cmd := exec.Command("systemd-run", "--unit=easynode-upgrade", "--setenv=HOME=/root", "--setenv=GOCACHE=/tmp/easynode-gocache", "--setenv=GOMODCACHE=/tmp/easynode-gomodcache", "bash", "-lc", "curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode --skip-upgrade --skip-bbr")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.setUpgrade(100, "upgrade failed", string(out), err.Error(), backup, false)
		return
	}
	s.setUpgrade(70, "upgrade task started", string(out), "", backup, true)
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "80", "--no-pager").CombinedOutput()
		s.setUpgrade(70+i, "installing update", string(statusOut), "", backup, true)
		if !upgradeUnitActive() {
			break
		}
	}
	statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "120", "--no-pager").CombinedOutput()
	result, code := upgradeUnitResult()
	if result != "success" || code != "0" {
		s.setUpgrade(100, "upgrade failed", string(statusOut), "upgrade task failed: result="+result+" status="+code, backup, false)
		return
	}
	s.setUpgrade(100, "upgrade complete, refreshing panel", string(statusOut), "", backup, false)
}

func upgradeUnitActive() bool {
	return exec.Command("systemctl", "is-active", "--quiet", "easynode-upgrade").Run() == nil
}

func upgradeUnitResult() (string, string) {
	out, err := exec.Command("systemctl", "show", "easynode-upgrade", "-p", "Result", "-p", "ExecMainStatus", "--value").CombinedOutput()
	if err != nil {
		return "unknown", "unknown"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := "unknown"
	code := "unknown"
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		result = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		code = strings.TrimSpace(lines[1])
	}
	return result, code
}

func systemdUpgradeStatus() upgradeState {
	out, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "120", "--no-pager").CombinedOutput()
	logs := string(out)
	active := exec.Command("systemctl", "is-active", "--quiet", "easynode-upgrade").Run() == nil
	result, code := upgradeUnitResult()
	st := upgradeState{
		Running:   active,
		Progress:  0,
		Step:      "waiting",
		Output:    logs,
		UpdatedAt: time.Now(),
	}
	if active {
		st.Progress = inferUpgradeProgress(logs)
		st.Step = "installing update"
		return st
	}
	if strings.Contains(logs, "EasyNode installed.") || (result == "success" && code == "0") {
		st.Progress = 100
		st.Step = "upgrade complete, refreshing panel"
		return st
	}
	if result != "unknown" && result != "" && result != "success" {
		st.Progress = 100
		st.Step = "upgrade failed"
		st.Error = "upgrade task failed: result=" + result + " status=" + code
		return st
	}
	return upgradeState{}
}

func inferUpgradeProgress(logs string) int {
	progress := 10
	markers := []struct {
		text     string
		progress int
	}{
		{"[1/8]", 15},
		{"[2/8]", 25},
		{"[3/8]", 40},
		{"[4/8]", 52},
		{"[5/8]", 65},
		{"[6/8]", 78},
		{"[7/8]", 88},
		{"[8/8]", 95},
	}
	for _, m := range markers {
		if strings.Contains(logs, m.text) {
			progress = m.progress
		}
	}
	return progress
}

func (s *Server) setUpgrade(progress int, step, output, errText, backup string, running bool) {
	s.upgrade.mu.Lock()
	defer s.upgrade.mu.Unlock()
	s.upgrade.Progress = progress
	s.upgrade.Step = step
	if output != "" {
		s.upgrade.Output = output
	}
	s.upgrade.Error = errText
	s.upgrade.Backup = backup
	s.upgrade.Running = running
	s.upgrade.UpdatedAt = time.Now()
}

func (s *Server) upgradeSnapshot() upgradeState {
	s.upgrade.mu.RLock()
	defer s.upgrade.mu.RUnlock()
	return s.upgrade
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
		st.CertReady = certReady
		st.CertPath = certPath
		st.KeyPath = keyPath
		host := req.Domain
		if req.IPDirect || host == "" {
			host = "127.0.0.1"
		}
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
				if !protocolRunnable(st.Nodes[i].Protocol, st.CertReady) {
					return errors.New(protocolUnavailableReason(st.Nodes[i].Protocol))
				}
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
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
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

func (s *Server) SubscribeQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	st := s.store.Snapshot()
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	writeQRCode(w, scheme+"://"+host+"/api/v1/subscribe/"+st.SubscribeKey)
}

func (s *Server) NodeQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/qrcode/node/")
	st := s.store.Snapshot()
	for _, n := range st.Nodes {
		if n.ID == id {
			link := n.SubscribeLink
			if link == "" && n.Status == "running" {
				link = subscribe.Link(n)
			}
			if link == "" {
				writeError(w, http.StatusBadRequest, errors.New("node link unavailable"))
				return
			}
			writeQRCode(w, link)
			return
		}
	}
	http.NotFound(w, r)
}

func writeQRCode(w http.ResponseWriter, text string) {
	png, err := qrcode.Encode(text, qrcode.Medium, 260)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(png)
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
	st := s.store.Snapshot()
	if st.ChainPairingDisabled {
		writeError(w, http.StatusForbidden, errors.New("chain pairing is disabled on this server"))
		return
	}
	outboundLink := firstRunningLink(st.Nodes)
	if outboundLink == "" {
		writeError(w, http.StatusBadRequest, errors.New("no running node available for chain exit"))
		return
	}
	code := chain.NewCode(req.Endpoint, outboundLink)
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
	if bundle, err := chain.DecodeBundle(strings.TrimSpace(req.Code)); err == nil {
		name := chainPeerName(req.DisplayName, bundle.Endpoint)
		err := s.store.Update(func(st *model.AppState) error {
			upsertChainPeer(st, model.ChainPeer{ID: util.Token(6), Name: name, Endpoint: bundle.Endpoint, PublicKey: bundle.PublicKey, OutboundLink: bundle.OutboundLink, Status: "paired", CreatedAt: time.Now()})
			return nil
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
		_ = singbox.RestartService()
		notified, notifyErr := notifyExitPaired(bundle, req.Endpoint, req.PublicKey)
		writeJSON(w, http.StatusOK, map[string]any{"peer_public_key": bundle.PublicKey, "peer_endpoint": bundle.Endpoint, "tunnel_config": bundle, "exit_notified": notified, "exit_notify_error": notifyErr})
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
		name := chainPeerName(req.DisplayName, c.Endpoint)
		upsertChainPeer(st, model.ChainPeer{ID: util.Token(6), Name: name, Endpoint: c.Endpoint, PublicKey: c.PublicKey, OutboundLink: c.OutboundLink, Status: "paired", CreatedAt: time.Now()})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer_public_key": paired.PublicKey, "peer_endpoint": paired.Endpoint, "tunnel_config": paired})
}

func (s *Server) RemoveChainPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var removedPeer model.ChainPeer
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainPeers[:0]
		removed := false
		for _, peer := range st.ChainPeers {
			if peer.ID == req.ID {
				removedPeer = peer
				removed = true
				continue
			}
			next = append(next, peer)
		}
		if !removed {
			return errors.New("chain peer not found")
		}
		st.ChainPeers = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	selfEndpoint := chainSelfEndpoint(st)
	notified, notifyErr := notifyRemoteUnpaired(removedPeer.Endpoint, removedPeer.Endpoint, selfEndpoint, "", "")
	resp := publicState(st)
	out := withPanelURL(resp)
	out["exit_notified"] = notified
	if notifyErr != "" {
		out["exit_notify_error"] = notifyErr
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) RemoveChainClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var removed model.ChainClient
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainClients[:0]
		found := false
		for _, client := range st.ChainClients {
			if client.ID == req.ID {
				removed = client
				found = true
				continue
			}
			next = append(next, client)
		}
		if !found {
			return errors.New("chain client not found")
		}
		st.ChainClients = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	notified, notifyErr := notifyEntryUnpaired(removed, chainSelfEndpoint(st))
	resp := publicState(st)
	out := withPanelURL(resp)
	out["entry_notified"] = notified
	if notifyErr != "" {
		out["entry_notify_error"] = notifyErr
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) SetChainAccepting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Accepting bool `json:"accepting"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	err := s.store.Update(func(st *model.AppState) error {
		st.ChainPairingDisabled = !req.Accepting
		if st.ChainPairingDisabled {
			st.PairingCodes = nil
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, publicState(s.store.Snapshot()))
}

func upsertChainPeer(st *model.AppState, peer model.ChainPeer) {
	for i := range st.ChainPeers {
		if st.ChainPeers[i].Endpoint == peer.Endpoint || (peer.OutboundLink != "" && st.ChainPeers[i].OutboundLink == peer.OutboundLink) {
			if st.ChainPeers[i].ID != "" {
				peer.ID = st.ChainPeers[i].ID
			}
			if !st.ChainPeers[i].CreatedAt.IsZero() {
				peer.CreatedAt = st.ChainPeers[i].CreatedAt
			}
			st.ChainPeers[i] = peer
			return
		}
	}
	st.ChainPeers = append(st.ChainPeers, peer)
}

func chainPeerMatches(peer model.ChainPeer, endpoint, publicKey, outboundLink string) bool {
	return (endpoint != "" && peer.Endpoint == endpoint) ||
		(publicKey != "" && peer.PublicKey == publicKey) ||
		(outboundLink != "" && peer.OutboundLink == outboundLink)
}

func chainClientMatches(client model.ChainClient, endpoint, publicKey string) bool {
	return (endpoint != "" && client.Endpoint == endpoint) ||
		(publicKey != "" && client.PublicKey == publicKey)
}

func upsertChainClient(st *model.AppState, client model.ChainClient) {
	for i := range st.ChainClients {
		if st.ChainClients[i].Endpoint == client.Endpoint || (client.PublicKey != "" && st.ChainClients[i].PublicKey == client.PublicKey) {
			if st.ChainClients[i].ID != "" {
				client.ID = st.ChainClients[i].ID
			}
			if !st.ChainClients[i].CreatedAt.IsZero() {
				client.CreatedAt = st.ChainClients[i].CreatedAt
			}
			st.ChainClients[i] = client
			return
		}
	}
	st.ChainClients = append(st.ChainClients, client)
}

func notifyExitPaired(bundle chain.Bundle, endpoint, publicKey string) (bool, string) {
	u, err := url.Parse(bundle.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, "invalid exit endpoint"
	}
	u.Path = "/api/v1/chain/public/paired"
	u.RawQuery = ""
	body, _ := json.Marshal(map[string]string{"code": bundle.Code, "endpoint": endpoint, "public_key": publicKey, "outbound_link": bundle.OutboundLink})
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(u.String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	if resp == nil {
		return false, "empty response"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "exit returned " + resp.Status
	}
	return true, ""
}

func notifyEntryUnpaired(client model.ChainClient, exitEndpoint string) (bool, string) {
	return notifyRemoteUnpaired(client.Endpoint, exitEndpoint, client.Endpoint, client.PublicKey, client.OutboundLink)
}

func notifyRemoteUnpaired(remoteEndpoint, exitEndpoint, entryEndpoint, publicKey, outboundLink string) (bool, string) {
	u, err := url.Parse(remoteEndpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, "invalid remote endpoint"
	}
	u.Path = "/api/v1/chain/public/unpaired"
	u.RawQuery = ""
	body, _ := json.Marshal(map[string]string{"exit_endpoint": exitEndpoint, "endpoint": entryEndpoint, "public_key": publicKey, "outbound_link": outboundLink})
	httpClient := http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Post(u.String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	if resp == nil {
		return false, "empty response"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "entry returned " + resp.Status
	}
	return true, ""
}

func chainSelfEndpoint(st model.AppState) string {
	if !st.IPDirect && st.Domain != "" && st.CertReady {
		return "https://" + st.Domain + ":8443"
	}
	if st.Domain != "" {
		return "http://" + st.Domain + ":8088"
	}
	return ""
}

func chainPeerName(displayName, endpoint string) string {
	if displayName != "" && len(displayName) <= 40 && !strings.HasPrefix(displayName, "Exit ENPAIR-") {
		return displayName
	}
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		return "Exit " + u.Hostname()
	}
	if endpoint != "" && len(endpoint) <= 40 {
		return "Exit " + endpoint
	}
	return "Chain exit"
}

func chainClientName(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		return "Entry " + u.Hostname()
	}
	if endpoint != "" && len(endpoint) <= 40 {
		return "Entry " + endpoint
	}
	return "Entry server"
}

func firstRunningLink(nodes []model.Node) string {
	for _, n := range nodes {
		if n.Status == "running" {
			if n.SubscribeLink != "" {
				return n.SubscribeLink
			}
			if link := subscribe.Link(n); link != "" {
				return link
			}
		}
	}
	return ""
}

func (s *Server) SingBoxConfig(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	path, err := singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
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

func nodesFromRecommendations(recs []model.Recommendation, selected []string, domain string, ipDirect bool, certReady bool) []model.Node {
	want := map[string]bool{}
	for _, p := range selected {
		want[p] = true
	}
	host := domain
	if ipDirect || host == "" {
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
	ports := map[string]int{"vless-reality": 443, "hysteria2": 8443, "trojan-tls": 2053, "vless-ws-tls": 2083, "tuic": 9443}
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
	if !protocolRunnable(n.Protocol, certReady) {
		n.Status = "stopped"
	}
	if n.Status == "running" {
		n.SubscribeLink = subscribe.Link(n)
	}
	return n
}

func protocolRunnable(protocol string, certReady bool) bool {
	if protocol == "vless-reality" {
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
			if protocolRunnable(n.Protocol, st.CertReady) {
				if n.Status == "running" {
					n.SubscribeLink = subscribe.Link(*n)
				}
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
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
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
