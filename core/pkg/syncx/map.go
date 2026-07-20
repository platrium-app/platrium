package syncx

import "sync"

// Map wraps the standard library's sync.Map to provide compile-time type safety
// for read-heavy, low-write concurrent operations.
type Map[K comparable, V any] struct {
	internal sync.Map
}

// NewMap instantiates a type-safe concurrent map.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{}
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.internal.Store(key, value)
}

// Load returns the value stored in the map for a key, or the zero value if no value is present.
func (m *Map[K, V]) Load(key K) (V, bool) {
	val, ok := m.internal.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value. The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	actual, loaded := m.internal.LoadOrStore(key, value)
	return actual.(V), loaded
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	val, ok := m.internal.LoadAndDelete(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.internal.Delete(key)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.internal.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
