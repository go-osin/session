package pqstore

import (
	"database/sql"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq" // postgresql driver

	"github.com/go-osin/session"
	"github.com/go-osin/session/codec"
)

type storeImpl struct {
	codec codec.Codec // Codec used to marshal and unmarshal a Session to a byte slice
	db    *sql.DB     // database conn
	// Map of sessions (mapped from ID) that were accessed using this store; usually it will only be 1.
	// It is also used as a cache, should the user call Get() with the same id multiple times.
	sessions map[string]session.Session

	mux *sync.RWMutex // mutex to synchronize access to sessions
}

// StoreOptions ...
type StoreOptions struct {
	DSN   string
	Codec *codec.Codec
}

// NewStore return instance with default Option
func NewStore() session.Store {
	opt := &StoreOptions{
		DSN: os.Getenv("SESSION_STORE_DSN"),
	}
	return NewStoreOptions(opt)
}

// NewStoreOptions return instance with special option
func NewStoreOptions(o *StoreOptions) session.Store {
	db, err := sql.Open("postgres", o.DSN)
	if err != nil {
		log.Printf("open db fail: %s", err)
		return nil
	}

	if o.Codec == nil {
		o.Codec = &codec.JSON
	}

	s := &storeImpl{
		codec:    *o.Codec,
		db:       db,
		sessions: make(map[string]session.Session, 2),
		mux:      &sync.RWMutex{},
	}
	s.createSessionsTable()
	return s
}

type sessionImpl struct {
	IDF      string                 `json:"id"`      // ID of the session
	CreatedF time.Time              `json:"created"` // Creation time
	CAttrsF  map[string]interface{} `json:"cattrs"`  // Constant attributes specified at session creation
	AttrsF   map[string]interface{} `json:"attrs"`   // Attributes stored in the session
}

type sessionEntry struct {
	Key       string
	Data      []byte
	Created   time.Time
	Modified  time.Time
	ExpiresAt time.Time
}

// Load is to implement Store.Load().
func (s *storeImpl) Load(id string) session.Session {
	s.mux.RLock()
	defer s.mux.RUnlock()
	// First check our "cache"
	if sess := s.sessions[id]; sess != nil {
		sess.Access()
		return sess
	}

	se, err := s.dbGet(id)
	if err != nil {
		log.Printf("failed to get from db: %s", err)
		return nil
	}

	var sess sessionImpl

	err = s.codec.Unmarshal(se.Data, &sess)
	if err != nil {
		log.Printf("failed to unmarshal session data: %s", err)
		return nil
	}

	ss := session.NewSessionOptions(&session.SessOptions{
		IDF:      sess.IDF,
		CreatedF: sess.CreatedF,
		CAttrs:   sess.CAttrsF,
		Attrs:    sess.AttrsF,
	})
	ss.Access()
	// log.Printf("session loaded from db, id: %s, vals %v", sess.IDF, sess.AttrsF)
	s.sessions[id] = ss
	return ss
}

// Save is to implement Store.Save().
func (s *storeImpl) Save(sess session.Session) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if s.storeSession(sess) {
		// log.Printf("Session save to db: %s", sess.ID())
		s.sessions[sess.ID()] = sess
		return
	}
}

// storeSession sets the specified session in the database.
func (s *storeImpl) storeSession(sess session.Session) bool {
	data, err := s.codec.Marshal(sess)
	if err != nil {
		log.Printf("failed to marshal session: %s", err)
		return false
	}
	// log.Printf("session: %s", data)
	now := time.Now()
	se := &sessionEntry{
		Key:       sess.ID(),
		Data:      data,
		Created:   sess.Created(),
		Modified:  now,
		ExpiresAt: now.Add(sess.Timeout()),
	}

	err = s.dbStore(se)
	if err != nil {
		log.Printf("failed to store session to cache, id: %s, error: %v", sess.ID(), err)
		return false
	}

	return true
}

// Remove is to implement Store.Remove().
func (s *storeImpl) Remove(sess session.Session) {
	s.mux.Lock()
	defer s.mux.Unlock()

	delete(s.sessions, sess.ID())

	err := s.dbDelete(sess.ID())
	if err != nil {
		log.Printf("failed to remove session from s.Codec, id: %s, error: %v", sess.ID(), err)
		return
	}
	// log.Printf("session removed: %s", sess.ID())
}

// Close is to implement Store.Close().
func (s *storeImpl) Close() {
	// Flush out sessions that were accessed from this store. No need locking, we're closing...
	// We could use Codec.SetMulti(), but sessions will contain at most 1 session like all the times.
	for _, sess := range s.sessions {
		s.storeSession(sess)
	}
	s.db.Close()
}

func (s *storeImpl) createSessionsTable() error {
	stmt := `
		CREATE TABLE IF NOT EXISTS http_sessions (
		key TEXT,
		data BYTEA,
		created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		modified TIMESTAMPTZ,
		expires_at TIMESTAMPTZ,
		PRIMARY KEY (key));
		`

	_, err := s.db.Exec(stmt)
	if err != nil {
		log.Printf("failed to create table: %s", err)
		return err
	}

	return nil
}

func (s *storeImpl) dbGet(id string) (se *sessionEntry, err error) {
	stmt := "SELECT data, created, modified, expires_at FROM http_sessions WHERE key = $1"
	se = &sessionEntry{Key: id}
	err = s.db.QueryRow(stmt, id).Scan(&se.Data, &se.Created, &se.Modified, &se.ExpiresAt)
	return
}

func (s *storeImpl) dbStore(se *sessionEntry) error {
	stmt := `INSERT INTO http_sessions(key, data, expires_at) VALUES($1, $2, $3)
	ON CONFLICT (key) DO UPDATE SET data = $4, modified = now() `
	_, err := s.db.Exec(stmt, se.Key, se.Data, se.ExpiresAt, se.Data)
	return err
}

func (s *storeImpl) dbDelete(id string) error {
	stmt := `DELETE FROM http_sessions WHERE key = $1`
	_, err := s.db.Exec(stmt, id)
	return err
}
