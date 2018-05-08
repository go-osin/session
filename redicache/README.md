SessionStore implementation with redis cache
===

Usage
---

```go
import "github.com/go-osin/session/redicache"
import "github.com/go-osin/session"
```

```go

	var smgr session.Manager
	var store session.Store

	store = redicache.NewRedicacheStoreOptions(&redicache.RedicacheStoreOptions{
		Servers: []string{":6379"}
	})
	smgr = session.NewCookieManagerOptions(store, &session.CookieMngrOptions{
		SessIDCookieName: SessionIDCookieName,
		AllowHTTP:        true,
	})


```
