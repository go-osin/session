package redicache

import (
	"encoding/gob"
	"testing"
	"time"

	"github.com/icza/mighty"

	"github.com/go-osin/session"
)

type Namer interface {
	GetName() string
}

type vect struct {
	Name string
}

func (v *vect) GetName() string {
	return v.Name
}

var (
	_ Namer = (*vect)(nil)
)

func TestRedicacheStore(t *testing.T) {
	gob.Register(&vect{})
	eq, neq := mighty.EqNeq(t)

	st := NewStore()
	defer st.Close()

	eq(nil, st.Load("asdf"))

	s := session.NewSession()
	var v Namer
	v = &vect{Name: "name"}
	s.Set("test", v.(Namer))
	st.Save(s)
	time.Sleep(15 * time.Millisecond)
	s_ := st.Load(s.ID())
	// eq(s, s_)
	value := s_.Get("test")
	eq(v.GetName(), value.(Namer).GetName())
	eq(len(s.Values()), len(s_.Values()))
	neq(s_.Accessed(), s_.Created())

	// st.Remove(s)
	// eq(nil, st.Load(s.ID()))
}
