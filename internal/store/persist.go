package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// snapshot — состояние приложения на диске. Формат версионируется полем V.
type snapshot struct {
	V         int                        `json:"v"`
	SavedAt   time.Time                  `json:"saved_at"`
	Actions   map[string]Absence         `json:"actions"`
	Checklist map[string][]ChecklistItem `json:"checklist"`
	Seq       map[string]int64           `json:"seq"`
	Gaps      map[string]map[string]*Gap `json:"gaps"`
	Sessions  map[string]Session         `json:"sessions"`
	Messages  map[string]Message         `json:"messages"`
	BySess    map[string][]string        `json:"by_session"`
}

const snapshotVersion = 1

// Persister сохраняет оба хранилища в один JSON-файл и восстанавливает их при старте.
// Это замена БД для прототипа: перезапуск сервера не теряет заявки, чек-листы и диалоги.
type Persister struct {
	Path  string
	Store *Store
	Msgs  *MessageStore
	saved int64
}

func (p *Persister) version() int64 { return p.Store.Version() + p.Msgs.Version() }

// Load читает снимок. Отсутствие файла — не ошибка (первый запуск).
func (p *Persister) Load() error {
	raw, err := os.ReadFile(p.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return err
	}
	if snap.V != snapshotVersion {
		return errors.New("unsupported snapshot version")
	}

	s := p.Store
	s.mu.Lock()
	if snap.Actions != nil {
		s.absences = snap.Actions
	}
	if snap.Checklist != nil {
		s.checklist = snap.Checklist
	}
	if snap.Seq != nil {
		s.seq = snap.Seq
	}
	if snap.Gaps != nil {
		s.gaps = snap.Gaps
	}
	s.mu.Unlock()

	m := p.Msgs
	m.mu.Lock()
	if snap.Sessions != nil {
		m.sessions = snap.Sessions
	}
	if snap.Messages != nil {
		m.messages = snap.Messages
	}
	if snap.BySess != nil {
		m.bySess = snap.BySess
	}
	// Ответы, которые не успели завершиться до остановки, уже никто не досчитает.
	for id, msg := range m.messages {
		if msg.Role == "assistant" && (msg.Status == "pending" || msg.Status == "in_progress") {
			msg.Status = "failed"
			msg.Content = "Сервер был перезапущен, отправьте вопрос ещё раз."
			m.messages[id] = msg
		}
	}
	m.mu.Unlock()

	p.saved = p.version()
	return nil
}

// Save пишет снимок атомарно: во временный файл и переименованием.
func (p *Persister) Save() error {
	ver := p.version()
	s, m := p.Store, p.Msgs
	s.mu.RLock()
	m.mu.RLock()
	raw, err := json.Marshal(snapshot{
		V: snapshotVersion, SavedAt: time.Now(),
		Actions: s.absences, Checklist: s.checklist, Seq: s.seq, Gaps: s.gaps,
		Sessions: m.sessions, Messages: m.messages, BySess: m.bySess,
	})
	m.mu.RUnlock()
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		return err
	}
	tmp := p.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p.Path); err != nil {
		return err
	}
	p.saved = ver
	return nil
}

// Run сохраняет состояние раз в interval, если оно изменилось, и ещё раз — при остановке.
func (p *Persister) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := p.Save(); err != nil {
				slog.Error("state save on shutdown failed", "err", err)
			}
			return
		case <-t.C:
			if p.version() == p.saved {
				continue
			}
			if err := p.Save(); err != nil {
				slog.Error("state save failed", "err", err)
			}
		}
	}
}
