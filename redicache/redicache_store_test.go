package redicache

import (
	"testing"
	"time"

	"github.com/icza/mighty"

	"github.com/go-osin/session"
)

func TestRedicacheStore(t *testing.T) {
	eq, neq := mighty.EqNeq(t)

	st := NewStore()
	defer st.Close()

	eq(nil, st.Load("asdf"))

	s := session.NewSession()
	s.Set("test", "value")
	st.Save(s)
	time.Sleep(15 * time.Millisecond)
	s_ := st.Load(s.ID())
	// eq(s, s_)
	value := s_.Get("test")
	eq("value", value.(string))
	eq(len(s.Values()), len(s_.Values()))
	neq(s_.Accessed(), s_.Created())

	st.Remove(s)
	eq(nil, st.Load(s.ID()))
}
