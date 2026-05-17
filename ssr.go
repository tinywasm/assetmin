package assetmin

import (
	"bytes"
	"fmt"
)

// EnableSSRMode activates the SSR event branch unconditionally. Pure setter.
func (c *AssetMin) EnableSSRMode() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ssrEnabled = true
}

// SetSSRCompiler registers a Go compiler callback. Pure setter — does NOT invoke fn.
// Pass nil to unregister.
func (c *AssetMin) SetSSRCompiler(fn func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSSRCompile = fn
}

// FlushToDisk snapshots all registered assets, writes them to disk (overwrite),
// and sets diskMirrored = true only on full success. Returns the first write error.
func (c *AssetMin) FlushToDisk() error {
	type snapshot struct {
		path    string
		content []byte
	}

	c.mu.Lock()
	snapshots := make([]snapshot, 0, len(c.allAssets))
	for _, a := range c.allAssets {
		a.RegenerateCache(c.activeMinifier())
		snapshots = append(snapshots, snapshot{
			path:    a.outputPath,
			content: a.GetCachedMinified(),
		})
	}
	c.mu.Unlock()

	for _, s := range snapshots {
		if err := FileWrite(s.path, *bytes.NewBuffer(s.content)); err != nil {
			return fmt.Errorf("FlushToDisk %s: %w", s.path, err)
		}
	}

	c.mu.Lock()
	c.diskMirrored = true
	c.mu.Unlock()
	return nil
}

// isSSRMode returns true if the package is being used as a dependency (SSR mode).
// It assumes the caller holds c.mu.
func (c *AssetMin) isSSRMode() bool {
	return c.ssrEnabled
}
