package cache

import (
	"container/list"
	"sync"
	"time"
)

const defaultMaximumSize = 1<<31 - 1

// currentTime is an alias for time.Now, used for testing.
var currentTime = time.Now

// localCache implements LoadingCache.
type localCache struct {
	maximumSize int
	onRemoval   OnRemoval

	cacheMu sync.RWMutex
	cache   map[Key]*list.Element

	entriesMu sync.Mutex
	entries   list.List
}

// newLocalCache returns a default localCache
func newLocalCache() *localCache {
	c := &localCache{
		maximumSize: defaultMaximumSize,
		cache:       make(map[Key]*list.Element),
	}
	c.entries.Init()
	return c
}

// entry stores cached entry key and value.
type entry struct {
	key   Key
	value Value

	lastAccess time.Time
}

// GetIfPresent gets cached value from entries list and updates
// last access time for the entry if it is found.
func (c *localCache) GetIfPresent(k Key) (Value, bool) {
	c.cacheMu.RLock()
	el, hit := c.cache[k]
	c.cacheMu.RUnlock()
	if !hit {
		return nil, false
	}

	// Put this element to the top
	c.entriesMu.Lock()
	en := el.Value.(*entry)
	en.lastAccess = currentTime()
	v := en.value
	c.entries.MoveToFront(el)
	c.entriesMu.Unlock()
	return v, true
}

// Put adds new entry to entries list.
func (c *localCache) Put(k Key, v Value) {
	c.cacheMu.RLock()
	el, hit := c.cache[k]
	c.cacheMu.RUnlock()
	if hit {
		// Update list element value
		c.entriesMu.Lock()
		en := el.Value.(*entry)
		en.value = v
		en.lastAccess = currentTime()
		c.entries.MoveToFront(el)
		c.entriesMu.Unlock()
		return
	}

	var remEn *entry
	en := &entry{
		key:        k,
		value:      v,
		lastAccess: currentTime(),
	}
	c.cacheMu.Lock()
	c.entriesMu.Lock()
	// Double check
	el, hit = c.cache[k]
	if hit {
		// Replace list element value
		el.Value = en
		c.entries.MoveToFront(el)
	} else {
		// Add new element
		el = c.entries.PushFront(en)
		c.cache[k] = el
		if c.maximumSize > 0 && c.entries.Len() > c.maximumSize {
			remEn = c.removeOldest()
		}
	}
	c.entriesMu.Unlock()
	c.cacheMu.Unlock()
	if c.onRemoval != nil && remEn != nil {
		c.onRemoval(remEn.key, remEn.value)
	}
}

// Invalidate removes the entry associated with key k.
func (c *localCache) Invalidate(k Key) {
	c.cacheMu.Lock()
	el, hit := c.cache[k]
	if !hit {
		c.cacheMu.Unlock()
		return
	}
	c.entriesMu.Lock()

	c.entries.Remove(el)
	delete(c.cache, k)

	c.entriesMu.Unlock()
	c.cacheMu.Unlock()

	if c.onRemoval != nil {
		en := el.Value.(*entry)
		c.onRemoval(en.key, en.value)
	}
}

// InvalidateAll resets entries list.
func (c *localCache) InvalidateAll() {
	var oldCache map[Key]*list.Element

	c.cacheMu.Lock()
	c.entriesMu.Lock()

	oldCache = c.cache
	c.cache = make(map[Key]*list.Element)
	c.entries.Init()

	c.entriesMu.Unlock()
	c.cacheMu.Unlock()

	if c.onRemoval != nil {
		for _, el := range oldCache {
			en := el.Value.(*entry)
			c.onRemoval(en.key, en.value)
		}
	}
}

// removeOldest removes oldest element in entries list and returns removed entry.
// Calling this function must be guarded by entries and cache mutex.
func (c *localCache) removeOldest() *entry {
	el := c.entries.Back()
	if el == nil {
		return nil
	}
	c.entries.Remove(el)
	en := el.Value.(*entry)
	delete(c.cache, en.key)
	return en
}

// New returns a local in-memory Cache.
func New(options ...Option) Cache {
	c := newLocalCache()
	for _, opt := range options {
		opt(c)
	}
	return c
}

// Option add options for default Cache.
type Option func(c *localCache)

// WithMaximumSize returns an Option which sets maximum size for default Cache.
// Any non-positive numbers is considered as unlimited.
func WithMaximumSize(size int) Option {
	return func(c *localCache) {
		c.maximumSize = size
	}
}

// WithRemovalListener returns an Option to set cache to call onRemoval for each
// entry evicted from the cache.
func WithRemovalListener(onRemoval OnRemoval) Option {
	return func(c *localCache) {
		c.onRemoval = onRemoval
	}
}
