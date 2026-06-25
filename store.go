package flags

// Store is a durable backing for flags. When set via WithStore, the in-memory
// map acts as a write-through read cache and Set/Delete persist to the Store.
type Store interface {
	Save(Flag)
	Get(key string) (Flag, bool)
	All() []Flag
	Delete(key string)
}

// WithStore swaps in a durable Store. Flags already in the Store are loaded into
// the in-memory cache; subsequent Set/Delete write through to it. Returns the
// Service for chaining.
func (s *Service) WithStore(store Store) *Service {
	s.mu.Lock()
	s.store = store
	for _, f := range store.All() {
		ff := f
		s.flags[ff.Key] = &ff
	}
	s.mu.Unlock()
	return s
}
