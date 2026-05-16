package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"easynode/internal/model"
	"easynode/internal/util"
)

type Store struct {
	mu    sync.RWMutex
	path  string
	state model.AppState
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dataDir, "state.json")}
	if b, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, err
		}
		changed := false
		if s.state.PanelPath == "" {
			s.state.PanelPath = "/panel-" + util.Token(3)
			changed = true
		}
		if s.state.SubscribeKey == "" {
			s.state.SubscribeKey = util.Token(16)
			changed = true
		}
		if s.state.SessionTokenHash == "" {
			s.state.SessionTokenHash = util.SHA256Hex(util.Token(24))
			changed = true
		}
		if changed {
			if err := s.saveLocked(); err != nil {
				return nil, err
			}
		}
	} else if errors.Is(err, os.ErrNotExist) {
		s.state = model.AppState{
			PanelPath:        "/panel-" + util.Token(3),
			SubscribeKey:     util.Token(16),
			SessionTokenHash: util.SHA256Hex(util.Token(24)),
			UpdatedAt:        time.Now(),
		}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return s, nil
}

func (s *Store) Snapshot() model.AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Store) Update(fn func(*model.AppState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.state); err != nil {
		return err
	}
	s.state.UpdatedAt = time.Now()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
