/*

Session Store interface.

*/

package session

// Store is a session store interface.
// A session store is responsible to store sessions and make them retrievable by their IDs at the server side.
type Store interface {
	// Load returns the session specified by its id.
	// The returned session will have an updated access time (set to the current time).
	// nil is returned if this store does not contain a session with the specified id.
	Load(id string) Session

	// Save adds a new session to the store.
	Save(sess Session)

	// Remove removes a session from the store.
	Remove(sess Session)

	// Close closes the session store, releasing any resources that were allocated.
	Close()
}
