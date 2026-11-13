package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // user | assistant
	Content   string    `json:"content"`
	Status    string    `json:"status"`           // pending | in_progress | done | failed
	Result    string    `json:"result,omitempty"` // JSON ответа агента
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	messages map[string]Message
	bySess   map[string][]string
	version  atomic.Int64
}

func NewMessageStore() *MessageStore {
	return &MessageStore{
		sessions: map[string]Session{},
		messages: map[string]Message{},
		bySess:   map[string][]string{},
	}
}

// Version — счётчик изменений (для сохранения на диск).
func (s *MessageStore) Version() int64 { return s.version.Load() }

func (s *MessageStore) CreateSession(userID, id string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := Session{ID: id, UserID: userID, CreatedAt: time.Now()}
	s.sessions[id] = sess
	s.version.Add(1)
	return sess
}

func (s *MessageStore) GetSession(id string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *MessageStore) CreateMessage(m Message) Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[m.ID] = m
	s.bySess[m.SessionID] = append(s.bySess[m.SessionID], m.ID)
	s.version.Add(1)
	return m
}

func (s *MessageStore) UpdateMessage(id string, fn func(*Message)) (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.messages[id]
	if !ok {
		return Message{}, false
	}
	fn(&m)
	s.messages[id] = m
	s.version.Add(1)
	return m, true
}

func (s *MessageStore) GetMessage(id string) (Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.messages[id]
	return m, ok
}

func (s *MessageStore) MessagesBySession(sessionID string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.bySess[sessionID]
	out := make([]Message, 0, len(ids))
	for _, id := range ids {
		if m, ok := s.messages[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

// PendingCount — сколько ответов помощника пользователь сейчас ждёт в сессии (ограничение параллельных запусков).
func (s *MessageStore) PendingCount(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, id := range s.bySess[sessionID] {
		if m := s.messages[id]; m.Role == "assistant" && (m.Status == "pending" || m.Status == "in_progress") {
			n++
		}
	}
	return n
}
