package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
