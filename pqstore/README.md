SessionStore implementation with PostgreSQL
===

Usage
---

```go
import "github.com/go-osin/session/pqstore"
import "github.com/go-osin/session"
```

```go

	var smgr session.Manager
	var store session.Store

	store = pqstore.NewStoreOptions(&pqstore.StoreOptions{
		DSN: "host=localhost dbname=ssotest user=sso sslmode=disable" // or "postgresql://sso@localhost/ssotest?sslmode=disable"
	})
	smgr = session.NewCookieManagerOptions(store, &session.CookieMngrOptions{
		SessIDCookieName: SessionIDCookieName,
		AllowHTTP:        true,
	})


```
