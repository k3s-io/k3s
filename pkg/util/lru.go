package util

import "k8s.io/utils/lru"

// Cache is a generic wrapper around lru.Cache that handles type assertions when
// retrieving cached entries.
type Cache[T any] struct {
	cache *lru.Cache
}

func NewCache[T any](size int) *Cache[T] {
	return &Cache[T]{
		cache: lru.New(size),
	}
}

func NewCacheWithEvictionFunc[T any](size int, f lru.EvictionFunc) *Cache[T] {
	return &Cache[T]{
		cache: lru.NewWithEvictionFunc(size, f),
	}
}

func (c *Cache[T]) Add(key lru.Key, value T) {
	c.cache.Add(key, value)
}

func (c *Cache[T]) Clear() {
	c.cache.Clear()
}

func (c *Cache[T]) Get(key lru.Key) (value T, ok bool) {
	v, ok := c.cache.Get(key)
	if !ok {
		return value, ok
	}
	value, ok = v.(T)
	return value, ok
}

func (c *Cache[T]) Len() int {
	return c.cache.Len()
}

func (c *Cache[T]) Remove(key lru.Key) {
	c.cache.Remove(key)
}

func (c *Cache[T]) RemoveOldest() {
	c.cache.RemoveOldest()
}

func (c *Cache[T]) SetEvictionFunc(f lru.EvictionFunc) {
	c.cache.SetEvictionFunc(f)
}
