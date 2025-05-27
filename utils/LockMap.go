package utils

import (
	"sync"
)

type Map[K, V any] struct {
	m    map[*K]*V
	lock sync.RWMutex
}

func (m *Map[K, V]) Get(key *K) *V {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if v, ok := m.m[key]; ok {
		return v
	}
	return nil
}

func (m *Map[K, V]) Put(key *K, value *V) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.m == nil {
		m.m = make(map[*K]*V)
	}
	if _, ok := m.m[key]; ok {
		return false
	}
	m.m[key] = value
	return true
}

func (m *Map[K, V]) Remove(key *K) *V {
	m.lock.Lock()
	defer m.lock.Unlock()
	if value, ok := m.m[key]; ok {
		delete(m.m, key)
		return value
	}
	return nil
}
