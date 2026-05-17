package assetmin

import (
	"bytes"
)

// EnableSSRMode switches NewFileEvent into the SSR event-handling branch
// (module-keyed slot updates via ReloadSSRModule). Independent of any compiler.
// Idempotent. There is no DisableSSRMode — SSR mode is a one-way activation
// for a session.
func (c *AssetMin) EnableSSRMode() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ssrEnabled = true
}

// SetSSRCompiler registers the optional .go-event compiler hook.
// PURE SETTER: it stores fn and returns. It does NOT invoke fn.
// Passing nil unregisters the compiler (events for .go files become no-ops).
func (c *AssetMin) SetSSRCompiler(fn func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSSRCompile = fn
}

// FlushToDisk writes every in-memory asset to its outputPath, overwriting any
// existing file. On full success, the AssetMin transitions into disk-mirrored
// state (subsequent in-memory mutations are also written to disk). On any
// per-asset write error, returns the first error and does NOT enter
// disk-mirrored state — the caller decides whether to abort the transition.
func (c *AssetMin) FlushToDisk() error {
	type snapshot struct {
		path    string
		content []byte
	}

	c.mu.Lock()
	var snapshots []snapshot
	for _, a := range c.allAssets {
		// Regenerate cache under lock
		if err := a.RegenerateCache(c.activeMinifier()); err != nil {
			c.mu.Unlock()
			return err
		}
		snapshots = append(snapshots, snapshot{
			path:    a.outputPath,
			content: a.GetCachedMinified(),
		})
	}
	c.mu.Unlock()

	// Write without holding the lock
	for _, s := range snapshots {
		if err := FileWrite(s.path, *bytes.NewBuffer(s.content)); err != nil {
			return err
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
