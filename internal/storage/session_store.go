//Store Thought Process(存储思维历程)

package storage

import (
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
}

// 结构体
type InMemorySessionStore struct {
	sessions map[string]*models.Session
	mutex    sync.RWMutex
}

type FileSessionStore struct {
	dataDir string
	mutex   sync.RWMutex
}

// 函数
func NewInMemorySessionStore() SessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*models.Session),
	}
}

func NewFileSessionStore(dataDir string) SessionStore {
	if dataDir == "" {
		dataDir = "data/sessions"
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(fmt.Sprintf("failed to create session data directory: %v", err))
	}

	return &FileSessionStore{dataDir: dataDir}
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

	return writeSessionFile(path, session)
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
	return writeSessionFile(path, session)
}

func (store *FileSessionStore) Delete(sessionID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	path := store.sessionPath(sessionID)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (store *FileSessionStore) GetByUserID(userID string) ([]*models.Session, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	sessions := make([]*models.Session, 0)
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

		if session.UserID == userID {
			sessions = append(sessions, session)
		}
		return nil
	})

	return sessions, err
}

func (store *FileSessionStore) GetExpiredSessions(before time.Time) ([]*models.Session, error) {
	sessions := make([]*models.Session, 0)
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

		if session.UpdatedAt.Before(before) {
			sessions = append(sessions, session)
		}
		return nil
	})

	return sessions, err
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
