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
			ctx = ContextWithSession(ctx, sess)
			defer func() {
				if sess, ok := FromContext(r.Context()); ok {
					if sess.Changed() {
						mgr.Save(sess, w)
					}
				}
			}()
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// ContextWithSession returns a new Context that carries value Session.
func ContextWithSession(ctx context.Context, sess Session) context.Context {
	return context.WithValue(ctx, SessionKey, sess)
}

// FromContext return Session in a request context
func FromContext(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(SessionKey).(Session)
	return sess, ok
}
