# Stage 3 — Dynamic Minification Toggle (DevTUI HandlerExecution)

**Goal:** `AssetMin` implements the `devtui.HandlerExecution` interface so that the TUI shows a toggle button that enables/disables minification at runtime. Toggling regenerates all assets immediately (and rewrites disk files if `buildOnDisk` is true).

**Requires:** Stage 1 and Stage 2 completed.

---

## Problem

There is no way to disable minification at runtime from the DevTUI. Developers debugging CSS or JS need to inspect readable output, but currently must restart the server with a different config or read minified content.

---

## Interface Contract

From `tinywasm/devtui/interfaces.go`:

```go
type HandlerExecution interface {
    Name() string   // identifier — already implemented, returns "ASSETS"
    Label() string  // button label shown in TUI — must be dynamic
    Execute()       // called on button press — no parameters, no return
}
```

`AssetMin` already implements `Name()`. This stage adds `Label()` and `Execute()`.

---

## Design

### New field in `AssetMin`

Add a boolean field to track minification state:

```go
type AssetMin struct {
    // ... existing fields ...
    minifyEnabled bool // true by default
}
```

Initialize to `true` in `NewAssetMin`.

### New file: `assetmin/minify_toggle.go`

```go
// Label returns the TUI button label reflecting current minification state.
func (c *AssetMin) Label() string {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.minifyEnabled {
        return "Minify: ON"
    }
    return "Minify: OFF"
}

// Execute toggles minification and regenerates all assets.
// If buildOnDisk is true, all asset files on disk are rewritten.
func (c *AssetMin) Execute() {
    c.mu.Lock()
    c.minifyEnabled = !c.minifyEnabled
    c.mu.Unlock()

    c.regenerateAll()
}
```

### `regenerateAll` (internal)

Regenerates all 5 asset handlers using current minification state:

```go
func (c *AssetMin) regenerateAll() {
    handlers := []*asset{
        c.mainStyleCssHandler,
        c.mainJsHandler,
        c.spriteSvgHandler,
        c.faviconSvgHandler,
        c.indexHtmlHandler,
    }
    for _, h := range handlers {
        _ = c.processAsset(h)
    }
}
```

### Minifier state

When `minifyEnabled == false`, pass `nil` as the minifier to `RegenerateCache` instead of `c.min`. `RegenerateCache` must handle a `nil` minifier by returning raw unminified content without panicking.

**Crucial Fix for `asset.go`:**
Currently, `RegenerateCache` and `GetMinifiedContent` call `minifier.Bytes(h.mediatype, buf.Bytes())`. If `minifier` is `nil`, this causes a panic (nil pointer dereference). You must update both methods in `asset.go` to safely handle a `nil` minifier:

```go
// In RegenerateCache and GetMinifiedContent:
if minifier == nil {
    h.cachedMinified = buf.Bytes()
    h.cacheValid = true
    return nil // or return h.cachedMinified, nil for GetMinifiedContent
}
```

The `processAsset` method calls `fh.RegenerateCache(c.min)`. Modify `AssetMin` to use the active minifier:

```go
func (c *AssetMin) activeMinifier() *minify.M {
    if c.minifyEnabled {
        return c.min
    }
    return nil
}
```

Then use `c.activeMinifier()` in `processAsset`:

```go
func (c *AssetMin) processAsset(fh *asset) error {
    if err := fh.RegenerateCache(c.activeMinifier()); err != nil {
        return err
    }
    if c.buildOnDisk {
        return FileWrite(fh.outputPath, *bytes.NewBuffer(fh.GetCachedMinified()))
    }
    return nil
}
```

**No log on toggle** — the TUI label itself communicates the state change visually.

---

## Files to Create / Modify

| File | Action |
|------|--------|
| `assetmin/minify_toggle.go` | **Create** — `Label()`, `Execute()`, `regenerateAll()`, `activeMinifier()` |
| `assetmin/assetmin.go` | **Modify** — add `minifyEnabled bool` field, initialize to `true` in `NewAssetMin` |
| `assetmin/events.go` | **Modify** — `processAsset` uses `c.activeMinifier()` instead of `c.min` directly |
| `assetmin/asset.go` | **Modify** — `RegenerateCache` and `GetMinifiedContent` explicitly check `if minifier == nil` |

---

## Tests — `assetmin/tests/minify_toggle_test.go`

### `TestMinifyToggle_DefaultIsOn`
On creation, minification is enabled.

```
given: NewAssetMin(config)
when:  Label()
then:  returns "Minify: ON"
```

### `TestMinifyToggle_ToggleOffChangesLabel`
```
given: NewAssetMin(config)
when:  Execute()
then:  Label() returns "Minify: OFF"
```

### `TestMinifyToggle_ToggleOnChangesLabel`
Double toggle returns to ON.

```
given: NewAssetMin(config) with Execute() called once
when:  Execute() called again
then:  Label() returns "Minify: ON"
```

### `TestMinifyToggle_AssetsRegeneratedWithoutMinification`
After toggling off, CSS content is human-readable (not minified). This also verifies the `nil` panic fix.

```
given: AssetMin with a CSS file containing "  .foo  {  color: red;  }"
       minification ON: result is ".foo{color:red}"
when:  Execute() (toggle off)
then:  CSS output contains the original whitespace or at least is NOT ".foo{color:red}"
       (i.e., minifier was not applied)
```

### `TestMinifyToggle_AssetsRegeneratedWithMinification`
After toggle on again, CSS is minified.

```
given: same setup, toggle was off
when:  Execute() (toggle back on)
then:  CSS output is minified form ".foo{color:red}"
```

### `TestMinifyToggle_DiskRewriteWhenBuildOnDisk`
When `buildOnDisk == true`, toggling rewrites the output files on disk.

```
given: AssetMin with buildOnDisk=true, temp outputDir
       a CSS file registered with content "  body { margin: 0; }"
when:  Execute() (toggle off)
then:  disk file at outputDir/style.css contains unminified content
       (verify by reading the file after Execute)
when:  Execute() again (toggle on)
then:  disk file contains minified content
```

### `TestMinifyToggle_MemoryOnlyWhenNotBuildOnDisk`
When `buildOnDisk == false`, no disk write occurs.

```
given: AssetMin with buildOnDisk=false, temp outputDir
when:  Execute()
then:  outputDir/style.css does NOT exist (or is not rewritten)
       in-memory content is still updated
```

---

## Acceptance Criteria

- [ ] All tests in `minify_toggle_test.go` pass
- [ ] `AssetMin` satisfies `devtui.HandlerExecution` interface (compile-time check via `var _ devtui.HandlerExecution = (*AssetMin)(nil)`)
- [ ] `Label()` reflects current state dynamically without lock contention
- [ ] `Execute()` regenerates all 5 asset handlers
- [ ] Disk files rewritten when `buildOnDisk == true`
- [ ] No log emitted on toggle — TUI label communicates state
- [ ] `RegenerateCache(nil)` and `GetMinifiedContent(nil)` return raw unminified content safely without panicking
