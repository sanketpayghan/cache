package cache

import (
	"container/list"
	"sync"
	"time"
)

// entry stores cached entry key and value.
type entry struct {
	key   Key
	value Value

	// accessed is the last time this entry was accessed.
	accessed time.Time
	// updated is the last time this entry was updated.
	updated time.Time
	// listID is ID of the list which this entry is currently in.
	listID listID
	// hash is the hash value of this entry key
	hash uint64

	//
	refreshMu sync.Mutex
	isRefreshing bool

}

// getEntry returns the entry attached to the given list element.
func getEntry(el *list.Element) *entry {
	return el.Value.(*entry)
}

// setEntry updates value of the given list element.
func setEntry(el *list.Element, en *entry) {
	el.Value = en
}

// lockEntry locks entry for refreshing so that no subsequent call will do refresh on same entry.
func (en *entry) lockEntry() bool {
	en.refreshMu.Lock()
	canRefresh := !en.isRefreshing
	en.isRefreshing = true
	en.refreshMu.Unlock()
	return canRefresh
}

func (en *entry) unlockEntry()  {
	en.refreshMu.Lock()
	en.isRefreshing = false
	en.refreshMu.Unlock()
}

// cache is a data structure for cache entries.
type cache struct {
	mu   sync.RWMutex
	data map[Key]*list.Element
}

// policy is a cache policy.
type policy interface {
	init(cache *cache, maximumSize int)
	add(newEntry *entry) *entry
	hit(element *list.Element)
	remove(element *list.Element) *entry
	walk(func(list *list.List))
}

func newPolicy(name string) policy {
	switch name {
	case "", "slru":
		return &slruCache{}
	case "lru":
		return &lruCache{}
	case "tinylfu":
		return &tinyLFU{}
	default:
		panic("cache: unsupported policy " + name)
	}
}
