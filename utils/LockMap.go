package utils

import (
	"sync"
)

type Map[K comparable, V any] struct {
	m    map[K]V
	lock sync.RWMutex
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *Map[K, V]) Put(key K, value V) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.m == nil {
		m.m = make(map[K]V)
	}
	if _, ok := m.m[key]; ok {
		return false
	}
	m.m[key] = value
	return true
}

func (m *Map[K, V]) Remove(key K) (V, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	value, ok := m.m[key]
	if ok {
		delete(m.m, key)
	}
	return value, ok
}
