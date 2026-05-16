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
	"strings"
	"sync"
	"time"

	"easynode/internal/core/cert"
	"easynode/internal/model"
	"easynode/internal/store"
	"easynode/internal/util"
)

type Server struct {
	store   *store.Store
	dataDir string
	static  embed.FS
	mux     *http.ServeMux
	upgrade upgradeState
	build   BuildInfo
	metrics *metricsCache
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
	s.metrics = startMetricsLoop(st)
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
	s.mux.HandleFunc("/api/v1/qrcode/clash", s.auth(s.ClashQRCode))
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
	s.mux.Handle("/.well-known/acme-challenge/", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir(cert.ChallengeDir()))))
	s.mux.HandleFunc("/", s.Static)
}

func (s *Server) enrichedState() model.AppState {
	return s.metrics.enrich(s.store.Snapshot())
}

func fetchRemotePairingStatus(url string) (bool, time.Time, *int) {
	client := http.Client{Timeout: 2 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)
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

func (s *Server) Static(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	if st.SetupDone && st.PanelPath != "" && r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, st.PanelPath) && !publicStaticPath(r.URL.Path) {
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
	if path == "manifest.webmanifest" {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	}
	if strings.HasPrefix(path, "assets/") || path == "favicon.svg" || path == "apple-touch-icon.png" {
		w.Header().Set("Cache-Control", "public, max-age=604800")
	} else if path == "sw.js" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(b))
}

func publicStaticPath(path string) bool {
	return strings.HasPrefix(path, "/assets/") ||
		path == "/manifest.webmanifest" ||
		path == "/sw.js" ||
		path == "/favicon.svg" ||
		path == "/apple-touch-icon.png"
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("easynode_session"); err == nil {
			st := s.store.Snapshot()
			if st.SessionTokenHash != "" && util.SHA256Hex(c.Value) == st.SessionTokenHash {
				next(w, r)
				return
			}
		}
		writeError(w, http.StatusUnauthorized, errors.New("login required"))
	}
}

