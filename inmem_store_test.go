package session

import (
	"testing"
	"time"

	"github.com/icza/mighty"
)

func TestInMemStore(t *testing.T) {
	eq, neq := mighty.EqNeq(t)

	st := NewInMemStore()
	defer st.Close()

	eq(nil, st.Load("asdf"))

	s := NewSession()
	st.Save(s)
	time.Sleep(10 * time.Millisecond)
	eq(s, st.Load(s.ID()))
	neq(s.Accessed(), s.Created())

	st.Remove(s)
	eq(nil, st.Load(s.ID()))
}

func TestInMemStoreSessCleaner(t *testing.T) {
	eq := mighty.Eq(t)

	st := NewInMemStoreOptions(&InMemStoreOptions{SessCleanerInterval: 10 * time.Millisecond})
	defer st.Close()

	s := NewSessionOptions(&SessOptions{Timeout: 50 * time.Millisecond})
	st.Save(s)
	eq(s, st.Load(s.ID()))

	time.Sleep(30 * time.Millisecond)
	eq(s, st.Load(s.ID()))

	time.Sleep(80 * time.Millisecond)
	eq(nil, st.Load(s.ID()))
}
