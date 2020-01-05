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

// JSON ...
var JSON = codec.JSON

type redicacheStore struct {
	keyPrefix string // Prefix to use in front of session ids to construct Redis key
	retries   int    // Number of retries to perform in case of general Redis failures

	codec *cache.Codec // Codec used to marshal and unmarshal a Session to a byte slice

	sessions map[string]session.Session

	mux *sync.RWMutex // mutex to synchronize access to sessions
}

type RedicacheStoreOptions struct {
	Addrs     []string
	DB        int
	Password  string
	KeyPrefix string
	Retries   int
	Codec     *codec.Codec
}

var zeroRedicacheStoreOptions = new(RedicacheStoreOptions)

func NewRedicacheStore() session.Store {
	return NewRedicacheStoreOptions(zeroRedicacheStoreOptions)
}

func NewRedicacheStoreOptions(o *RedicacheStoreOptions) session.Store {
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
	s := &redicacheStore{
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
	IDF      string                 // ID of the session
	CreatedF time.Time              // Creation time
	CAttrsF  map[string]interface{} // Constant attributes specified at session creation
	AttrsF   map[string]interface{} // Attributes stored in the session
}

// Load is to implement Store.Load().
func (s *redicacheStore) Load(id string) session.Session {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// First check our "cache"
	if sess, ok := s.sessions[id]; ok {
		sess.Access()
		return sess
	}

	// Next check in Memcache
	var err error
	var sess *sessionImpl

	key := s.keyPrefix + id
	for i := 0; i < s.retries; i++ {
		var sess_ sessionImpl
		err = s.codec.Get(key, &sess_)
		if err == cache.ErrCacheMiss {
			break // It's not in the cache
		}
		if err == nil {
			sess = &sess_
			break
		}
		// Service error? Retry..
	}

	if sess == nil {
		log.Printf("Failed to get session from redicache, id: %s, error: %v", id, err)
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
	return ss
}

// Save is to implement Store.Save().
func (s *redicacheStore) Save(sess session.Session) {
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.setCacheSession(sess) {
		log.Printf("Session redic saved: %s", sess.ID())
		s.sessions[sess.ID()] = sess
		return
	}
}

// setCacheSession sets the specified session in the Memcache.
func (s *redicacheStore) setCacheSession(sess session.Session) (success bool) {
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

	log.Printf("Failed to add session to cache, id: %s, error: %v", sess.ID(), err)
	return false
}

// Remove is to implement Store.Remove().
func (s *redicacheStore) Remove(sess session.Session) {
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
func (s *redicacheStore) Close() {
	// Flush out sessions that were accessed from this store. No need locking, we're closing...
	// We could use Codec.SetMulti(), but sessions will contain at most 1 session like all the times.
	for _, sess := range s.sessions {
		s.setCacheSession(sess)
	}
}
