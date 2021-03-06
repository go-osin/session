/*

Session interface and its implementation.

*/

package session

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"sync"
	"time"
)

// Session is the (HTTP) session interface.
// We can use it to store and retrieve constant and variable attributes from it.
type Session interface {
	// ID returns the id of the session.
	ID() string

	// New tells if the session is new.
	// Implementation is based on whether created and access times are equal.
	New() bool

	// Getp returns the value of an attribute provided at session creation.
	// These attributes cannot be changes during the lifetime of a session,
	// so they can be accessed safely without synchronization. Exampe is storing the
	// authenticated user.
	Getp(name string) interface{}

	// Get returns the value of an attribute stored in the session.
	// Safe for concurrent use.
	Get(name string) interface{}

	// Set sets the value of an attribute stored in the session.
	// Pass the nil value to delete the attribute.
	// Safe for concurrent use.
	Set(name string, value interface{})

	// Values returns a copy of all the attribute values stored in the session.
	// Safe for concurrent use.
	Values() map[string]interface{}

	// Created returns the session creation time.
	Created() time.Time

	// Accessed returns the time when the session was last accessed.
	Accessed() time.Time

	// Timeout returns the session timeout.
	// A session may be removed automatically if it is not accessed for this duration.
	Timeout() time.Duration

	// Mutex returns the RW mutex of the session.
	// It is used to synchronize access/modification of the state stored in the session.
	// It can be used if session-level synchronization is required.
	// Important! If Session values are marshalled / unmarshalled
	// (e.g. multi server instance environment such as Google AppEngine),
	// this mutex may be different for each Session value and thus
	// it can only be used to session-value level synchronization!
	Mutex() *sync.RWMutex

	// Access registers an access to the session,
	// updates its last accessed time to the current time.
	// Users do not need to call this as the session store is responsible for that.
	Access()

	// Changed return true if the session is Changed.
	Changed() bool
}

// Session implementation.
// Fields are exported so a session may be marshalled / unmarshalled.
type sessionImpl struct {
	IDF       string                 `json:"id"`       // ID of the session
	CreatedF  time.Time              `json:"created"`  // Creation time
	AccessedF time.Time              `json:"accessed"` // Last accessed time
	CAttrsF   map[string]interface{} `json:"cattrs"`   // Constant attributes specified at session creation
	AttrsF    map[string]interface{} `json:"attrs"`    // Attributes stored in the session
	TimeoutF  time.Duration          `json:"timeout"`  // Session timeout
	mux       *sync.RWMutex          // RW mutex to synchronize session state access
	changedF  bool
}

// SessOptions defines options that may be passed when creating a new Session.
// All fields are optional; default value will be used for any field that has the zero value.
type SessOptions struct {
	// ID of the session
	IDF string
	// Creation time
	CreatedF time.Time
	// Constant attributes of the session. These be will available via the Session.CAttr() method, without synchronization.
	// Values from the map will be copied, and will be available via Session.CAttr().
	CAttrs map[string]interface{}

	// Initial, non-constant attributes to be stored in the session.
	// Values from the map will be copied, and will be available via Session.Attr() and Session.Attrs,
	// and may be changed with Session.SetAttr().
	Attrs map[string]interface{}

	// Session timeout, default is 30 minutes.
	Timeout time.Duration

	// Byte-length of the information that builds up the session ids.
	// Using Base-64 encoding, id length will be this multiplied by 4/3 chars.
	// Default value is 18 (which means length of ID will be 24 chars).
	IDLength int
}

// Pointer to zero value of SessOptions to be reused for efficiency.
var zeroSessOptions = new(SessOptions)

// NewSession creates a new Session with the default options.
// Default values of options are listed in the SessOptions type.
func NewSession() Session {
	return NewSessionOptions(zeroSessOptions)
}

// NewSessionOptions creates a new Session with the specified options.
func NewSessionOptions(o *SessOptions) Session {
	now := time.Now()
	idLength := o.IDLength
	if idLength <= 0 {
		idLength = 18
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	var id string
	if o.IDF != "" {
		id = o.IDF
	} else {
		id = genID(idLength)
	}
	var created time.Time
	if !o.CreatedF.IsZero() {
		created = o.CreatedF
	} else {
		created = now
	}
	sess := sessionImpl{
		IDF:       id,
		CreatedF:  created,
		AccessedF: now,
		AttrsF:    make(map[string]interface{}),
		TimeoutF:  timeout,
		mux:       &sync.RWMutex{},
	}

	if len(o.CAttrs) > 0 {
		sess.CAttrsF = make(map[string]interface{}, len(o.CAttrs))
		for k, v := range o.CAttrs {
			sess.CAttrsF[k] = v
		}
	}

	for k, v := range o.Attrs {
		sess.AttrsF[k] = v
	}

	return &sess
}

// genID generates a secure, random session id using the crypto/rand package.
func genID(length int) string {
	r := make([]byte, length)
	io.ReadFull(rand.Reader, r)
	return base64.URLEncoding.EncodeToString(r)
}

// ID is to implement Session.ID().
func (s *sessionImpl) ID() string {
	return s.IDF
}

// New is to implement Session.New().
func (s *sessionImpl) New() bool {
	return s.CreatedF == s.AccessedF
}

// Changed is to implement Session.Changed().
func (s *sessionImpl) Changed() bool {
	return s.changedF
}

// Getp is to implement Session.Getp().
func (s *sessionImpl) Getp(name string) interface{} {
	return s.CAttrsF[name]
}

// Get is to implement Session.Get().
func (s *sessionImpl) Get(name string) interface{} {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.AttrsF[name]
}

// Set is to implement Session.Set().
func (s *sessionImpl) Set(name string, value interface{}) {
	s.mux.Lock()
	defer s.mux.Unlock()

	if value == nil {
		delete(s.AttrsF, name)
	} else {
		s.AttrsF[name] = value
	}
	s.changedF = true
}

// Values is to implement Session.Values().
func (s *sessionImpl) Values() map[string]interface{} {
	s.mux.RLock()
	defer s.mux.RUnlock()

	m := make(map[string]interface{}, len(s.AttrsF))
	for k, v := range s.AttrsF {
		m[k] = v
	}
	return m
}

// Created is to implement Session.Created().
func (s *sessionImpl) Created() time.Time {
	return s.CreatedF
}

// Accessed is to implement Session.Accessed().
func (s *sessionImpl) Accessed() time.Time {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.AccessedF
}

// Timeout is to implement Session.Timeout().
func (s *sessionImpl) Timeout() time.Duration {
	return s.TimeoutF
}

// Mutex is to implement Session.Mutex().
func (s *sessionImpl) Mutex() *sync.RWMutex {
	return s.mux
}

// Access is to implement Session.Access().
func (s *sessionImpl) Access() {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.AccessedF = time.Now()
}
