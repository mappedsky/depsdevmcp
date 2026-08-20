// Package cache provides a concurrency-safe, bounded in-memory LRU cache.
package cache

import (
	"container/list"
	"sync"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

// LRU is a least-recently-used cache. Get and Add are safe for concurrent use.
type LRU[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
}

// NewLRU creates an LRU that stores at most capacity entries.
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	if capacity <= 0 {
		panic("cache capacity must be positive")
	}

	return &LRU[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns a cached value and marks it as most recently used.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	c.order.MoveToFront(elem)
	return elem.Value.(entry[K, V]).value, true
}

// Add inserts or updates a value and evicts the least recently used entry when full.
func (c *LRU[K, V]) Add(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value = entry[K, V]{key: key, value: value}
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(entry[K, V]{key: key, value: value})
	c.items[key] = elem
	if c.order.Len() <= c.capacity {
		return
	}

	lru := c.order.Back()
	item := lru.Value.(entry[K, V])
	delete(c.items, item.key)
	c.order.Remove(lru)
}

// Delete removes key from the cache.
func (c *LRU[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		delete(c.items, key)
		c.order.Remove(elem)
	}
}

// Len returns the current number of entries.
func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Capacity returns the maximum number of entries.
func (c *LRU[K, V]) Capacity() int {
	return c.capacity
}
