package consensus

import (
	"sync"
	"time"
)

// HashStore manages the storage and retrieval of file hashes
// Thread-safe implementation for concurrent access
type HashStore struct {
	mu     sync.RWMutex
	hashes map[string]time.Time // hash -> timestamp when first seen
}

// NewHashStore creates a new hash store
func NewHashStore() *HashStore {
	return &HashStore{
		hashes: make(map[string]time.Time),
	}
}

// Has checks if a hash exists in the store
func (hs *HashStore) Has(hash string) bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	_, exists := hs.hashes[hash]
	return exists
}

// Store adds a hash to the store with current timestamp
func (hs *HashStore) Store(hash string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if _, exists := hs.hashes[hash]; !exists {
		hs.hashes[hash] = time.Now()
	}
}

// StoreWithTime adds a hash with a specific timestamp
func (hs *HashStore) StoreWithTime(hash string, t time.Time) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.hashes[hash] = t
}

// Count returns the number of hashes stored
func (hs *HashStore) Count() int {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return len(hs.hashes)
}

// GetAll returns all hashes (for debugging/monitoring)
func (hs *HashStore) GetAll() map[string]time.Time {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	result := make(map[string]time.Time, len(hs.hashes))
	for k, v := range hs.hashes {
		result[k] = v
	}
	return result
}

// CleanOld removes hashes older than the specified duration
// Useful for preventing unbounded memory growth
func (hs *HashStore) CleanOld(maxAge time.Duration) int {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for hash, timestamp := range hs.hashes {
		if timestamp.Before(cutoff) {
			delete(hs.hashes, hash)
			removed++
		}
	}

	return removed
}
