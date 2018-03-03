package session

import (
	"context"
	"net/http"
)

// Key to use when setting Session.
type ctxKeySession int

// SessionKey is the key that holds Session in a request context.
const SessionKey ctxKeySession = 0

type sessionFunc func() Session

// Middleware return a http middleware with session process
func Middleware(mgr Manager, sf sessionFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			sess := mgr.Load(r)
			if sess == nil {
				if sf == nil {
					sf = NewSession
				}
				sess = sf()
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, SessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
			if !sess.New() {
				mgr.Save(sess, w)
			}
		}
		return http.HandlerFunc(fn)
	}
}

// FromContext return Session in a request context
func FromContext(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(SessionKey).(Session)
	return sess, ok
}
