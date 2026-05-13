package assetmin

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ssrCacheEntry holds a cached extraction result keyed by module hash set.
type ssrCacheEntry struct {
	hashSet string                             // Combined hash of all module Go files
	results map[string]ssrCollectorOutput
}

// ssrCache manages content-hash based caching for SSR asset extraction.
type ssrCache struct {
	entries map[string]*ssrCacheEntry
}

// Global cache for SSR extraction results
var ssrGlobalCache = newSSRCache()

// newSSRCache creates a new cache instance.
func newSSRCache() *ssrCache {
	return &ssrCache{
		entries: make(map[string]*ssrCacheEntry),
	}
}

// computeModuleHashSet computes a combined hash of all Go files in the module set.
func computeModuleHashSet(modules []Module) (string, error) {
	var filePaths []string

	for _, m := range modules {
		if m.Dir == "" {
			continue
		}

		// Walk the module directory and collect all .go files (excluding *_test.go)
		err := filepath.Walk(m.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				filePaths = append(filePaths, path)
			}
			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to walk module dir %s: %w", m.Dir, err)
		}
	}

	// Sort for deterministic order
	sort.Strings(filePaths)

	// Compute hash of all file contents
	h := md5.New()
	for _, filePath := range filePaths {
		f, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", fmt.Errorf("failed to hash file %s: %w", filePath, err)
		}
		f.Close()
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// get retrieves cached results if the module hash set matches.
func (c *ssrCache) get(hashSet string) (map[string]ssrCollectorOutput, bool) {
	entry, ok := c.entries[hashSet]
	if !ok {
		return nil, false
	}
	return entry.results, true
}

// set caches extraction results for a module hash set.
func (c *ssrCache) set(hashSet string, results map[string]ssrCollectorOutput) {
	c.entries[hashSet] = &ssrCacheEntry{
		hashSet: hashSet,
		results: results,
	}
}
