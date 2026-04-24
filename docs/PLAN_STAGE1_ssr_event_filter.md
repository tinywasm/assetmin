# Stage 1 — SSR Event Filter & Hot-Reload Fallthrough

**Goal:** When SSR mode is active, `NewFileEvent` must handle `.go` and embedded asset files differently to avoid expensive WASM recompilations and asset duplication. `.go` files trigger `onSSRCompile`, while asset files trigger a local `ReloadSSRModule`.

**Blocks:** Stage 2 cannot start until all tests here pass.

---

## Problem

Current `NewFileEvent` in `events.go:49`:

```go
if c.isSSRMode() {
    return c.onSSRCompile()  // fires for ALL extensions, including .css, .js
}
```

This causes the external SSR compiler (usually a heavy WASM build) to run on every CSS or HTML file event, producing log noise and slow reloads.
However, letting non-`.go` files fall through to the normal `UpdateFileContentInMemory` flow would cause **asset duplication**, because the SSR module loads the asset under its `ModuleName` (e.g., `"components"`), while the file watcher would append it under its `filePath`.
The solution is to route `.go` files to the compiler, and for asset files, trigger `ReloadSSRModule` on their directory to safely overwrite the existing module's slot without duplicating it.

---

## Changes Required

### 1. `assetmin/events.go` — `NewFileEvent`

Modify the SSR guard to selectively route based on extension and module location:

```go
// BEFORE
if c.isSSRMode() {
    return c.onSSRCompile()
}
```

```go
// AFTER
if c.isSSRMode() {
    if extension == ".go" {
        return c.onSSRCompile() // Only .go files trigger full SSR recompilation
    }
    
    // The "Different Path": Hot-reload embedded files without rebuilding WASM
    // Only intercept valid asset extensions. Irrelevant files (.md, .txt) are ignored.
    switch extension {
    case ".css", ".js", ".svg", ".html":
        dir := filepath.Dir(filePath)
        if err := c.ReloadSSRModule(dir); err == nil {
            // Success: the module re-read its //go:embed files and updated its slot.
            // Regenerate cache and write to disk without duplicating memory blocks.
            // We only need to refresh the specific asset type that changed!
            c.RefreshAsset(extension)
            return nil
        }
    }

    // If ReloadSSRModule returns an error, it means no ssr.go exists in that dir.
    // Or if the extension is completely irrelevant, we ignore it.
    return nil
}
```

### 3. `assetmin/docs/SSR.md` — Update Documentation

Add a section explaining the dual-path event flow in SSR mode:
- `.go` file changes trigger `onSSRCompile()` (WASM rebuild).
- Asset file changes (`.css`, `.html`, etc.) in directories containing an `ssr.go` file will trigger an instant, localized module reload. This bypasses the WASM rebuild and safely overwrites the `AssetMin` memory cache.
- Loose asset files in directories without an `ssr.go` are completely ignored.

---

## Tests — `assetmin/tests/ssr_event_filter_test.go`

All tests must use table-driven style with `t.Run`.

### Test cases

#### `TestSSRMode_GoTriggersCompile`
SSR mode active. Assert that `.go` file events call `onSSRCompile`.

```
Setup:
1. Create AssetMin with mock onSSRCompile that sets compiled = true.
2. Call NewFileEvent("ssr.go", ".go", "/path/ssr.go", "write").
3. Assert: compiled == true.
```

#### `TestSSRMode_EmbeddedAssetHotReload`
SSR mode active. Assert that changing an embedded asset triggers a module reload, NOT a full compile.

```
Setup:
1. Create AssetMin in SSR mode with mock onSSRCompile (should NOT be called).
2. Create temp dir with an `ssr.go` and `style.css`.
3. Call NewFileEvent for `style.css`.
4. Assert: onSSRCompile was NOT called.
5. Assert: AssetMin CSS cache was updated with new content (ReloadSSRModule succeeded).
```

#### `TestSSRMode_LooseAssetIgnored`
SSR mode active. Asset without an `ssr.go` nearby is ignored.

```
Setup:
1. Create AssetMin in SSR mode.
2. Call NewFileEvent for a `.css` file in a directory without `ssr.go`.
3. Assert: returns nil.
4. Assert: cache is untouched.
```

#### `TestNonSSRMode_ProcessesAllEvents`
SSR mode NOT active. Existing behavior preserved.

```
Setup:
1. Create AssetMin without calling SetExternalSSRCompiler.
2. Call NewFileEvent with a .css file.
3. Assert: asset cache was updated normally.
```

---

## Acceptance Criteria

- [ ] All 4 test functions pass with `go test ./tests/...`
- [ ] `.go` events call `onSSRCompile` in SSR mode
- [ ] `.css`/`.js`/`.svg`/`.html` events in a module dir trigger `ReloadSSRModule` instead of `onSSRCompile`
- [ ] Loose assets are ignored in SSR mode
- [ ] Documentation updated to explain the hot-reload behavior
