//Store Thought Process(存储思维历程)

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/models"
)

// 接口
type SessionStore interface {
	Save(session *models.Session) error
	Get(sessionID string) (*models.Session, error)
	Update(session *models.Session) error
	Delete(sessionID string) error
	GetByUserID(userID string) ([]*models.Session, error)
	GetExpiredSessions(before time.Time) ([]*models.Session, error)
	Ping(ctx context.Context) error
}

// 结构体
type InMemorySessionStore struct {
	sessions map[string]*models.Session
	mutex    sync.RWMutex
}

type FileSessionStore struct {
	dataDir      string
	mutex        sync.RWMutex
	userIndex    map[string]map[string]struct{}
	sessionIndex map[string]sessionMetadata
}

type sessionMetadata struct {
	UpdatedAt time.Time
}

// 函数
func NewInMemorySessionStore() SessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*models.Session),
	}
}

func (store *InMemorySessionStore) Ping(ctx context.Context) error {
	return nil
}

func NewFileSessionStore(dataDir string) SessionStore {
	if dataDir == "" {
		dataDir = "data/sessions"
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(fmt.Sprintf("failed to create session data directory: %v", err))
	}

	store := &FileSessionStore{
		dataDir:      dataDir,
		userIndex:    make(map[string]map[string]struct{}),
		sessionIndex: make(map[string]sessionMetadata),
	}

	if err := store.buildIndex(); err != nil {
		panic(fmt.Sprintf("failed to build session index: %v", err))
	}

	return store
}

func (store *FileSessionStore) Ping(ctx context.Context) error {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	_, err := os.Stat(store.dataDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (store *FileSessionStore) buildIndex() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.userIndex = make(map[string]map[string]struct{})
	store.sessionIndex = make(map[string]sessionMetadata)

	err := filepath.WalkDir(store.dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		session, err := decodeSession(data)
		if err != nil {
			return err
		}
		store.indexSessionLocked(session)
		return nil
	})

	return err
}

// InMemorySessionStore方法
func (store *InMemorySessionStore) Save(session *models.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	if _, exists := store.sessions[session.ID]; exists {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	store.sessions[session.ID] = cloneSession(session)
	return nil
}

func (store *InMemorySessionStore) Get(sessionID string) (*models.Session, error) {
	store.mutex.RLock()
	session, ok := store.sessions[sessionID]
	store.mutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", appErrors.ErrSessionNotFound, sessionID)
	}

	return cloneSession(session), nil
}

func (store *InMemorySessionStore) Update(session *models.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	if _, exists := store.sessions[session.ID]; !exists {
		return fmt.Errorf("%w: %s", appErrors.ErrSessionNotFound, session.ID)
	}

	store.sessions[session.ID] = cloneSession(session)
	return nil
}

func (store *InMemorySessionStore) Delete(sessionID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	delete(store.sessions, sessionID)
	return nil
}

func (store *InMemorySessionStore) GetByUserID(userID string) ([]*models.Session, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	results := make([]*models.Session, 0)
	for _, session := range store.sessions {
		if session != nil && session.UserID == userID {
			results = append(results, cloneSession(session))
		}
	}
	return results, nil
}

func (store *InMemorySessionStore) GetExpiredSessions(before time.Time) ([]*models.Session, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	results := make([]*models.Session, 0)
	for _, session := range store.sessions {
		if session != nil && session.UpdatedAt.Before(before) {
			results = append(results, cloneSession(session))
		}
	}
	return results, nil
}

// FileSessionStore方法
func (store *FileSessionStore) Save(session *models.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	path := store.sessionPath(session.ID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	if err := writeSessionFile(path, session); err != nil {
		return err
	}

	store.indexSessionLocked(session)
	return nil
}

func (store *FileSessionStore) Get(sessionID string) (*models.Session, error) {
	store.mutex.RLock()
	path := store.sessionPath(sessionID)
	store.mutex.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", appErrors.ErrSessionNotFound, sessionID)
		}
		return nil, err
	}

	return decodeSession(data)
}

func (store *FileSessionStore) Update(session *models.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	path := store.sessionPath(session.ID)
	if err := writeSessionFile(path, session); err != nil {
		return err
	}

	store.indexSessionLocked(session)
	return nil
}

func (store *FileSessionStore) Delete(sessionID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	path := store.sessionPath(sessionID)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	store.removeFromIndexLocked(sessionID)
	return nil
}

func (store *FileSessionStore) GetByUserID(userID string) ([]*models.Session, error) {
	store.mutex.RLock()
	ids := store.lookupUserUnlocked(userID)
	store.mutex.RUnlock()

	sessions := make([]*models.Session, 0, len(ids))
	for _, id := range ids {
		session, err := store.Get(id)
		if err != nil {
			if errors.Is(err, appErrors.ErrSessionNotFound) {
				continue
			}
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (store *FileSessionStore) GetExpiredSessions(before time.Time) ([]*models.Session, error) {
	store.mutex.RLock()
	if store.sessionIndex == nil {
		store.mutex.RUnlock()
		return []*models.Session{}, nil
	}
	candidateIDs := make([]string, 0, len(store.sessionIndex))
	for id, meta := range store.sessionIndex {
		if meta.UpdatedAt.IsZero() || meta.UpdatedAt.Before(before) {
			candidateIDs = append(candidateIDs, id)
		}
	}
	store.mutex.RUnlock()

	if len(candidateIDs) == 0 {
		return []*models.Session{}, nil
	}

	result := make([]*models.Session, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		session, err := store.Get(id)
		if err != nil {
			if errors.Is(err, appErrors.ErrSessionNotFound) {
				continue
			}
			return nil, err
		}
		if session.UpdatedAt.Before(before) {
			result = append(result, session)
		}
	}

	return result, nil
}

func (store *FileSessionStore) sessionPath(sessionID string) string {
	return filepath.Join(store.dataDir, fmt.Sprintf("%s.json", sessionID))
}

func writeSessionFile(path string, session *models.Session) error {
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func decodeSession(data []byte) (*models.Session, error) {
	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	normalizeThoughtTree(session.RootThought, nil, nil)
	return &session, nil
}

func cloneSession(session *models.Session) *models.Session {
	if session == nil {
		return nil
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return nil
	}
	clone, err := decodeSession(payload)
	if err != nil {
		return nil
	}
	return clone
}

func normalizeThoughtTree(thought *models.Thought, parent *models.Thought, parentPath []string) {
	if thought == nil {
		return
	}

	if parentPath != nil {
		thought.Path = append(append([]string{}, parentPath...), thought.Content)
		pid := parent.ID
		thought.ParentID = &pid
	} else {
		thought.Path = []string{thought.Content}
		thought.ParentID = nil
	}

	if parent == nil {
		thought.Depth = 0
	} else {
		thought.Depth = parent.Depth + 1
	}

	for _, child := range thought.Children {
		normalizeThoughtTree(child, thought, thought.Path)
	}
}

func safeUpdatedAt(session *models.Session) time.Time {
	if session == nil {
		return time.Time{}
	}
	if !session.UpdatedAt.IsZero() {
		return session.UpdatedAt
	}
	if !session.CreatedAt.IsZero() {
		return session.CreatedAt
	}
	return time.Now().UTC()
}

func (store *FileSessionStore) indexSessionLocked(session *models.Session) {
	if session == nil {
		return
	}
	if store.userIndex == nil {
		store.userIndex = make(map[string]map[string]struct{})
	}
	if store.sessionIndex == nil {
		store.sessionIndex = make(map[string]sessionMetadata)
	}

	for userID, ids := range store.userIndex {
		if ids == nil {
			continue
		}
		if _, ok := ids[session.ID]; ok && userID != session.UserID {
			delete(ids, session.ID)
		}
	}

	if session.UserID == "" {
		store.sessionIndex[session.ID] = sessionMetadata{UpdatedAt: safeUpdatedAt(session)}
		return
	}

	ids := store.userIndex[session.UserID]
	if ids == nil {
		ids = make(map[string]struct{})
		store.userIndex[session.UserID] = ids
	}
	ids[session.ID] = struct{}{}
	store.sessionIndex[session.ID] = sessionMetadata{UpdatedAt: safeUpdatedAt(session)}
}

func (store *FileSessionStore) removeFromIndexLocked(sessionID string) {
	if store.userIndex == nil {
		return
	}
	for userID, ids := range store.userIndex {
		if ids == nil {
			continue
		}
		delete(ids, sessionID)
		if len(ids) == 0 {
			delete(store.userIndex, userID)
		}
	}
	if store.sessionIndex != nil {
		delete(store.sessionIndex, sessionID)
	}
}

func (store *FileSessionStore) lookupUserUnlocked(userID string) []string {
	if userID == "" || store.userIndex == nil {
		return nil
	}
	ids := store.userIndex[userID]
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	return result
}
