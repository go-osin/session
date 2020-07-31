package redicache

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/cache"
	"github.com/go-redis/redis"

	"github.com/go-osin/session"
	"github.com/go-osin/session/codec"
)

type storeImpl struct {
	keyPrefix string // Prefix to use in front of session ids to construct Redis key
	retries   int    // Number of retries to perform in case of general Redis failures

	codec *cache.Codec // Codec used to marshal and unmarshal a Session to a byte slice

	sessions map[string]session.Session

	mux *sync.RWMutex // mutex to synchronize access to sessions
}

// StoreOptions ...
type StoreOptions struct {
	Addrs     []string
	DB        int
	Password  string
	KeyPrefix string
	Retries   int
	Codec     *codec.Codec
}

var zeroStoreOptions = new(StoreOptions)

// NewStore ...
func NewStore() session.Store {
	return NewStoreOptions(zeroStoreOptions)
}

// NewStoreOptions ...
func NewStoreOptions(o *StoreOptions) session.Store {
	if len(o.Addrs) == 0 {
		o.Addrs = []string{":6379"}
	} else {
		log.Printf("redis addrs %v", o.Addrs)
	}
	var addrs = map[string]string{}
	for i, svr := range o.Addrs {
		k := fmt.Sprintf("server%d", i)
		addrs[k] = svr
	}
	ring := redis.NewRing(&redis.RingOptions{
		Addrs:    addrs,
		DB:       o.DB,
		Password: o.Password,
	})
	var cd codec.Codec
	if o.Codec == nil {
		cd = codec.Gob
	} else {
		cd = *o.Codec
	}
	codec := &cache.Codec{
		Redis:     ring,
		Marshal:   cd.Marshal,
		Unmarshal: cd.Unmarshal,
	}
	s := &storeImpl{
		keyPrefix: o.KeyPrefix,
		retries:   o.Retries,
		sessions:  make(map[string]session.Session, 2),
		mux:       &sync.RWMutex{},
	}
	if s.retries <= 0 {
		s.retries = 3
	}
	s.codec = codec

	return s
}

type sessionImpl struct {
	IDF      string                 `json:"id"`      // ID of the session
	CreatedF time.Time              `json:"created"` // Creation time
	CAttrsF  map[string]interface{} `json:"cattrs"`  // Constant attributes specified at session creation
	AttrsF   map[string]interface{} `json:"attrs"`   // Attributes stored in the session
}

// Load is to implement Store.Load().
func (s *storeImpl) Load(id string) session.Session {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Next check in Memcache
	var err error
	var sess *sessionImpl

	key := s.keyPrefix + id
	for i := 0; i < s.retries; i++ {
		var _sess sessionImpl
		err = s.codec.Get(key, &_sess)
		if err == cache.ErrCacheMiss {
			break // It's not in the cache
		}
		if err == nil {
			sess = &_sess
			break
		}
		// Service error? Retry..
	}

	if sess == nil {
		log.Printf("Failed to load session from redicache, id: %s, error: %v", id, err)
		return nil
	}

	ss := session.NewSessionOptions(&session.SessOptions{
		IDF:      sess.IDF,
		CreatedF: sess.CreatedF,
		CAttrs:   sess.CAttrsF,
		Attrs:    sess.AttrsF,
	})
	ss.Access()
	s.sessions[id] = ss
	log.Printf("session load from redic, id: %s, vals %v", sess.IDF, sess.AttrsF)
	return ss
}

// Save is to implement Store.Save().
func (s *storeImpl) Save(sess session.Session) {
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.storeSession(sess) {
		log.Printf("Session save to redic: %s", sess.ID())
		s.sessions[sess.ID()] = sess
		return
	}
}

// storeSession sets the specified session in the Memcache.
func (s *storeImpl) storeSession(sess session.Session) (success bool) {
	item := &cache.Item{
		Key:        s.keyPrefix + sess.ID(),
		Object:     sess,
		Expiration: sess.Timeout(),
	}

	var err error
	for i := 0; i < s.retries; i++ {
		if err = s.codec.Set(item); err == nil {
			return true
		}
	}

	log.Printf("Failed to store session to cache, id: %s, error: %v", sess.ID(), err)
	return false
}

// Remove is to implement Store.Remove().
func (s *storeImpl) Remove(sess session.Session) {
	s.mux.Lock()
	defer s.mux.Unlock()

	var err error
	for i := 0; i < s.retries; i++ {
		if err = s.codec.Delete(s.keyPrefix + sess.ID()); err == nil {
			log.Printf("Session redic removed: %s", sess.ID())
			delete(s.sessions, sess.ID())
			return
		}
	}
	log.Printf("Failed to remove session from s.Codec, id: %s, error: %v", sess.ID(), err)
}

// Close is to implement Store.Close().
func (s *storeImpl) Close() {
	// Flush out sessions that were accessed from this store. No need locking, we're closing...
	// We could use Codec.SetMulti(), but sessions will contain at most 1 session like all the times.
	for _, sess := range s.sessions {
		s.storeSession(sess)
	}
}
