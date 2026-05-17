# PLAN: Replace SetBuildOnDisk with FlushToDisk + EnableSSRMode + SetSSRCompiler

> Status: Ready for execution. Breaking change. No backwards compatibility shims.

## Why

`assetmin` has three bugs in its disk-write path, all rooted in a conflated API:

- **B1** — `processAssetSafe` skips writing if the file already exists (uses
  `FileWriteSafe`). On re-deploy, stale on-disk bytes are never overwritten.
- **B2** — `SetBuildOnDisk(true)` and `SetExternalSSRCompiler` only flush the 5
  main handlers; module CSS files tracked in other assets are silently skipped.
- **B3** — `isSSRMode()` returns `c.onSSRCompile != nil`. The only way to activate
  the SSR event-handling branch today is to pass a non-nil function, forcing
  callers to supply a fake no-op. `SetBuildOnDisk` and SSR mode are unrelated
  concerns that must be separated.

## New public API

```go
// EnableSSRMode activates the SSR event branch unconditionally. Pure setter.
func (c *AssetMin) EnableSSRMode()

// SetSSRCompiler registers a Go compiler callback. Pure setter — does NOT invoke fn.
// Pass nil to unregister.
func (c *AssetMin) SetSSRCompiler(fn func() error)

// FlushToDisk snapshots all registered assets, writes them to disk (overwrite),
// and sets diskMirrored = true only on full success. Returns the first write error.
func (c *AssetMin) FlushToDisk() error
```

## Removed API (no shims, no deprecated wrappers)

- `SetBuildOnDisk(onDisk bool)` — deleted.
- `SetExternalSSRCompiler(fn func() error, buildOnDisk bool)` — deleted.
- `processAssetSafe(fh *asset) error` — deleted.
- `BuildOnDisk() bool` — deleted.
- `FileWriteSafe` (filewrite.go) — deleted; `FileWrite` always overwrites.

## Implementation

### 1. assetmin.go — struct fields

Replace `buildOnDisk bool` with:

```go
ssrEnabled   bool
diskMirrored bool
allAssets    map[string]*asset // keyed by outputPath — dedup
```

In `NewAssetMin`, after creating the 5 handlers, register them:

```go
c.allAssets = make(map[string]*asset)
for _, a := range []*asset{
    c.mainStyleCssHandler, c.mainJsHandler,
    c.spriteSvgHandler, c.faviconSvgHandler, c.indexHtmlHandler,
} {
    c.allAssets[a.outputPath] = a
}
```

Remove `SetBuildOnDisk`, `processAssetSafe`, `BuildOnDisk`.

### 2. ssr.go — replace entirely

```go
package assetmin

import (
    "bytes"
    "fmt"
)

func (c *AssetMin) EnableSSRMode() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.ssrEnabled = true
}

func (c *AssetMin) SetSSRCompiler(fn func() error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.onSSRCompile = fn
}

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

func (c *AssetMin) isSSRMode() bool {
    return c.ssrEnabled
}
```

### 3. events.go — update processAsset

Replace `if c.buildOnDisk {` with `if c.diskMirrored {`.

In the SSR event path, add nil guard before calling `c.onSSRCompile()`:

```go
if c.onSSRCompile != nil {
    if err := c.onSSRCompile(); err != nil {
        c.Logger("SSR compile error:", err)
    }
}
```

### 4. filewrite.go — remove FileWriteSafe

Delete the `FileWriteSafe` function entirely. `FileWrite` already overwrites.

### 5. Existing tests

Tests calling `SetBuildOnDisk` or `SetExternalSSRCompiler` must be updated:

- `tests/assets_test.go` — replace `SetBuildOnDisk(true)` with `FlushToDisk()` + assert no error.
- `tests/minify_toggle_test.go` — replace `SetExternalSSRCompiler(nil, true)` with `EnableSSRMode()` + `FlushToDisk()`.
- `tests/ssr_test.go` — replace `SetExternalSSRCompiler(fn, true)` with `EnableSSRMode()` + `SetSSRCompiler(fn)` + `FlushToDisk()`.
- `tests/http_test.go` — replace `SetBuildOnDisk(false/true)` with `EnableSSRMode()` / `FlushToDisk()` as appropriate.

### 6. New tests (flush_to_disk_test.go)

The file `tests/flush_to_disk_test.go` already exists with reproducer tests all
marked `t.Skip`. Remove ALL `t.Skip` calls and replace each `// TODO(agent):` with
the real API call:

- `TestFlushToDisk_OverwritesStaleFile` → `env.AssetsHandler.FlushToDisk()`
- `TestFlushToDisk_WritesAllRegisteredAssets` → `env.AssetsHandler.FlushToDisk()`
- `TestFlushToDisk_ReturnsErrorOnWriteFailure` → construct unwritable OutputDir, assert error, assert `diskMirrored = false` (use `BuildOnDisk()` if still present; otherwise verify by checking that a subsequent `NewFileEvent` does NOT write to disk).
- `TestFlushToDisk_DedupesByOutputPath` → `env.AssetsHandler.FlushToDisk()`
- `TestDiskMirrored_AfterFlushPropagates` → `env.AssetsHandler.FlushToDisk()`
- `TestEnableSSRMode_StandaloneFlag` → `env.AssetsHandler.EnableSSRMode()`
- `TestSetSSRCompiler_DoesNotAutoInvoke` → `env.AssetsHandler.SetSSRCompiler(fn)`
- `TestSetSSRCompiler_NilUnregisters` → `env.AssetsHandler.EnableSSRMode()` + `SetSSRCompiler(fn)` + `SetSSRCompiler(nil)`

## Acceptance criteria

1. No call site references `SetBuildOnDisk`, `SetExternalSSRCompiler`, `processAssetSafe`, `BuildOnDisk`, or `FileWriteSafe`.
2. `go vet ./...` is clean.
3. `go build ./...` succeeds.
4. `go test ./...` passes — including all tests in `flush_to_disk_test.go` (no skips).
5. Coverage ≥ 75%.
