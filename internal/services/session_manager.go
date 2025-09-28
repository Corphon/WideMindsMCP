//Session Context Management(会话上下文管理)

package services

import (
	"context"
	"errors"
	"sync"
	"time"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/storage"
)

// 结构体
type SessionManager struct {
	store storage.SessionStore
	cache map[string]*models.Session
	mutex sync.RWMutex
}

// 函数
func NewSessionManager(store storage.SessionStore) *SessionManager {
	return &SessionManager{
		store: store,
		cache: make(map[string]*models.Session),
	}
}

// 方法
func (sm *SessionManager) CreateSession(userID, initialConcept string) (*models.Session, error) {
	if initialConcept == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	session := models.NewSession(userID, initialConcept)
	if err := sm.store.Save(session); err != nil {
		return nil, err
	}

	sm.mutex.Lock()
	sm.cache[session.ID] = session
	sm.mutex.Unlock()

	return session, nil
}

func (sm *SessionManager) GetSession(sessionID string) (*models.Session, error) {
	if sessionID == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	sm.mutex.RLock()
	session, ok := sm.cache[sessionID]
	sm.mutex.RUnlock()
	if ok {
		return session, nil
	}

	session, err := sm.store.Get(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, appErrors.ErrSessionNotFound
	}

	sm.mutex.Lock()
	sm.cache[sessionID] = session
	sm.mutex.Unlock()

	return session, nil
}

func (sm *SessionManager) UpdateSession(session *models.Session) error {
	if session == nil {
		return appErrors.ErrInvalidRequest
	}

	session.UpdatedAt = time.Now().UTC()
	if err := sm.store.Update(session); err != nil {
		return err
	}

	sm.mutex.Lock()
	sm.cache[session.ID] = session
	sm.mutex.Unlock()

	return nil
}

func (sm *SessionManager) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return appErrors.ErrInvalidRequest
	}

	if err := sm.store.Delete(sessionID); err != nil {
		return err
	}

	sm.mutex.Lock()
	delete(sm.cache, sessionID)
	sm.mutex.Unlock()

	return nil
}

func (sm *SessionManager) AddThoughtToSession(sessionID string, thought *models.Thought) error {
	if thought == nil {
		return appErrors.ErrInvalidRequest
	}

	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	thought.SessionID = session.ID

	if session.RootThought == nil {
		session.RootThought = thought
	} else {
		parent := session.RootThought
		if thought.ParentID != nil {
			tree := session.GetThoughtTree()
			if existing, ok := tree[*thought.ParentID]; ok {
				parent = existing
			}
		}

		if parent != nil {
			parent.AddChild(thought)
		} else {
			session.RootThought = thought
		}
	}

	return sm.UpdateSession(session)
}

func (sm *SessionManager) UpdateThought(sessionID, thoughtID string, update *models.ThoughtUpdate) (*models.Thought, error) {
	if update == nil {
		return nil, appErrors.ErrInvalidRequest
	}

	session, err := sm.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	thought, err := session.ApplyThoughtUpdate(thoughtID, update)
	if err != nil {
		return nil, err
	}

	if err := sm.store.Update(session); err != nil {
		return nil, err
	}

	sm.mutex.Lock()
	sm.cache[session.ID] = session
	sm.mutex.Unlock()

	return thought, nil
}

func (sm *SessionManager) DeleteThought(sessionID, thoughtID string) (*models.Session, error) {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if err := session.RemoveThought(thoughtID); err != nil {
		return nil, err
	}

	if err := sm.store.Update(session); err != nil {
		return nil, err
	}

	sm.mutex.Lock()
	sm.cache[session.ID] = session
	sm.mutex.Unlock()

	return session, nil
}

func (sm *SessionManager) GetActiveSessionsByUser(userID string) ([]*models.Session, error) {
	sessions, err := sm.store.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	active := make([]*models.Session, 0, len(sessions))
	for _, session := range sessions {
		if session != nil && session.IsActive {
			active = append(active, session)
		}
	}
	return active, nil
}

func (sm *SessionManager) CleanupExpiredSessions() error {
	threshold := time.Now().Add(-24 * time.Hour)
	sessions, err := sm.store.GetExpiredSessions(threshold)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session == nil {
			continue
		}
		if err := sm.DeleteSession(session.ID); err != nil {
			return err
		}
	}
	return nil
}

func (sm *SessionManager) HealthCheck(ctx context.Context) error {
	if sm == nil {
		return errors.New("session manager is nil")
	}
	if sm.store == nil {
		return errors.New("session store is nil")
	}
	return sm.store.Ping(ctx)
}
