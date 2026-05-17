# PLAN: Fix in-memory → disk transition on external server mode

> Status: Ready for execution. Breaking change. No backwards compatibility shims.
>
> Visual: [diagrams/FLUSH_TO_DISK.md](diagrams/FLUSH_TO_DISK.md) — buggy vs. expected flow.

## Context

`tinywasm/app` orchestrates `tinywasm/assetmin`, `tinywasm/client` and `tinywasm/server`.
While the **internal Go server** runs, `assetmin` serves assets from an in-memory cache
through routes registered at [assetmin/http.go:9](../http.go#L9).

When the user switches to **external server mode** (a `web/server.go` file is detected
and a separate process is spawned by [server/management.go:12-33](../../server/management.go#L12-L33)),
the external binary serves files from `web/public/` on disk. The expectation is that
every asset currently held in memory by `assetmin` is flushed to `web/public/` **before**
the external server starts. This is broken.

## Root cause (in this repository)

In [assetmin/ssr.go:11-23](../ssr.go#L11-L23), `SetExternalSSRCompiler(fn, true)` is the
transition trigger. It has four independent defects:

### B1 — `FileWriteSafe` skips existing files
[assetmin/assetmin.go:177-188](../assetmin.go#L177-L188) calls
`FileWriteSafe(fh.outputPath, ...)` which at [assetmin/filewrite.go:39-51](../filewrite.go#L39-L51)
**returns nil if the file already exists**. The current minified in-memory bytes are
therefore discarded whenever a stale file from a previous run is present on disk —
exactly the steady-state of a dev session.

### B2 — Only 5 hard-coded handlers are flushed
The transition flushes only `mainStyleCssHandler`, `mainJsHandler`, `spriteSvgHandler`,
`faviconSvgHandler`, `indexHtmlHandler`. Every other in-memory asset accumulated via
`UpdateFileContentInMemory` ([assetmin/events.go:108](../events.go#L108)) — per-module
CSS shards, additional SVGs, sub-files, fonts — is never written. The external server
serves 404s for them.

### B3 — Conflated API (`SetExternalSSRCompiler`)
The method mixes **four** concerns: enabling SSR event-handling mode, registering an SSR
compiler callback, toggling `buildOnDisk`, and performing a one-shot flush. Worse, the
SSR-mode flag is currently inferred from `onSSRCompile != nil`
([assetmin/ssr.go:28-30](../ssr.go#L28-L30)), so callers that need the SSR event-handling
branch in [events.go:53-71](../events.go#L53-L71) (module-keyed slot updates) are forced
to register a no-op compiler. See `app/docs/PLAN.md` B3 for the caller-side consequence.

### B4 — Deprecated alias still in tree
[assetmin/assetmin.go:171-175](../assetmin.go#L171-L175) keeps `SetBuildOnDisk(bool)` as
a deprecated shim. Tests under `assetmin/tests/` still call it, perpetuating the
conflated semantics.

## Breaking redesign (no patch, no shims)

### 1. Split the conflated API into single-responsibility methods

Delete `SetExternalSSRCompiler` **and** the deprecated `SetBuildOnDisk`. Introduce
**three** orthogonal methods (each with one job):

```go
// EnableSSRMode switches NewFileEvent into the SSR event-handling branch
// (module-keyed slot updates via ReloadSSRModule). Independent of any compiler.
// Idempotent. There is no DisableSSRMode — SSR mode is a one-way activation
// for a session.
func (c *AssetMin) EnableSSRMode()

// SetSSRCompiler registers the optional .go-event compiler hook.
// PURE SETTER: it stores fn and returns. It does NOT invoke fn.
// Passing nil unregisters the compiler (events for .go files become no-ops).
func (c *AssetMin) SetSSRCompiler(fn func() error)

// FlushToDisk writes every in-memory asset to its outputPath, overwriting any
// existing file. On full success, the AssetMin transitions into disk-mirrored
// state (subsequent in-memory mutations are also written to disk). On any
// per-asset write error, returns the first error and does NOT enter
// disk-mirrored state — the caller decides whether to abort the transition.
func (c *AssetMin) FlushToDisk() error
```

Internally, decouple the state:

```go
// in AssetMin struct (replacing buildOnDisk):
ssrEnabled   bool                 // set by EnableSSRMode
onSSRCompile func() error         // set by SetSSRCompiler (no auto-invoke)
diskMirrored bool                 // set ONLY on a fully successful FlushToDisk
```

`isSSRMode()` now returns `c.ssrEnabled`, not `c.onSSRCompile != nil`.

### 2. Disk-mirrored mode is implicit and post-flush

After a successful `FlushToDisk` sets `c.diskMirrored = true`, the per-event path in
[events.go:119-130](../events.go#L119-L130) (`processAsset`) must write through to disk
on every subsequent in-memory mutation using `FileWrite` (overwrite). There is no
public setter for `diskMirrored`; it is implied by having flushed successfully.

`FileWriteSafe` is deleted — there is no remaining caller and its semantics are wrong
for this domain.

### 3. Enumerate all assets via a deduplicated registry

Maintain `c.allAssets map[string]*asset` keyed by `outputPath`, populated wherever a
new asset is registered (`UpdateFileContentInMemory`, `addIcon`, the 5 main handlers
at construction). Using a map deduplicates: registering the same asset N times during
a session produces one entry, one disk write per `FlushToDisk` call. `FlushToDisk`
iterates the map.

### 4. Overwrite, do not skip

`FlushToDisk` uses `FileWrite` (truncating create) for every asset. Stale on-disk
content from a previous session must be replaced with current minified bytes.

### 5. Concurrency: snapshot-then-write

`FlushToDisk` must NOT hold `c.mu` during the I/O loop:

1. Acquire `c.mu`.
2. For every asset in `c.allAssets`, regenerate the minified cache and copy
   `(outputPath, bytes)` into a local slice.
3. Release `c.mu`.
4. Iterate the local slice and call `FileWrite` for each. Collect the first error.
5. Re-acquire `c.mu` only to set `c.diskMirrored = true` (iff no error).
6. Return error (or nil).

This prevents blocking concurrent HTTP serving in internal mode and avoids deadlock
with watcher-triggered event callbacks.

### 6. Tests to add (under `assetmin/tests/`)

Reproducer skeletons already committed in this PR
([../tests/flush_to_disk_test.go](../tests/flush_to_disk_test.go)) — all marked with
`t.Skip("see docs/PLAN.md")`. The agent **removes the skips and adapts to the new
API** as part of the implementation. They are the oracle.

Coverage matrix:

| Test                                          | Defect    | What it asserts                                                                  |
|-----------------------------------------------|-----------|----------------------------------------------------------------------------------|
| `TestFlushToDisk_OverwritesStaleFile`         | B1        | Stale on-disk bytes are replaced by current in-memory minified.                  |
| `TestFlushToDisk_WritesAllRegisteredAssets`   | B2        | All N>5 registered assets land on disk (CSS shards, extra SVGs).                 |
| `TestFlushToDisk_ReturnsErrorOnWriteFailure`  | new       | Write failure returns non-nil error AND `diskMirrored` stays false.              |
| `TestDiskMirrored_AfterFlushPropagates`       | B2 / §2   | After successful `FlushToDisk`, a subsequent in-memory mutation hits disk.       |
| `TestFlushToDisk_DedupesByOutputPath`         | §3        | Registering the same asset twice yields exactly one disk write per flush.        |
| `TestEnableSSRMode_StandaloneFlag`            | B3 / §1   | `EnableSSRMode()` activates the SSR event branch without a compiler set.         |
| `TestSetSSRCompiler_DoesNotAutoInvoke`        | B3 / §1   | `SetSSRCompiler(fn)` stores fn but does NOT invoke it. (Aligned with new spec.)  |
| `TestSetSSRCompiler_NilUnregisters`           | B3 / §1   | `SetSSRCompiler(nil)` clears the compiler; `.go` events become no-ops.           |

- Delete tests that only exercise the removed `SetBuildOnDisk` / `SetExternalSSRCompiler`
  signatures. Rewrite remaining tests against the new API.

## Cross-package contract (read-only for this PLAN)

`tinywasm/app` will call, at orchestrator init:

```go
h.AssetsHandler.EnableSSRMode()   // formerly the no-op SetExternalSSRCompiler call
// No SetSSRCompiler here — there is no real Go compiler in app.
```

And at every external-mode server start (synchronously, in order):

```go
h.WasmClient.UseDiskStorage()                      // switch client to disk mode
if err := h.WasmClient.Compile(); err != nil { /* abort */ }
if err := h.AssetsHandler.FlushToDisk(); err != nil { /* abort */ }
// only then: strategy.Start
```

Ordering rationale: `WasmClient.UseDiskStorage()` switches the client's mode flag,
which determines the **filename** of the wasm artifact that `assetmin` embeds into
`index.html` / `main.js` via `GetSSRClientInitJS()`. The dependency is on **client
state**, not disk I/O — assetmin does not read the wasm from disk.

(`tinywasm/client` is renaming `SetBuildOnDisk(bool, bool)` → `UseDiskStorage()` +
`UseMemoryStorage()` as part of this same refactor; see
[client PLAN](../../client/docs/PLAN.md).)

`FlushToDisk` is idempotent: subsequent external-mode restarts re-flush safely.

The matching PLAN.md in `tinywasm/app/docs/PLAN.md` covers the orchestrator side and
the `tinywasm/server` ripple (`OnExternalModeExecution` → `BeforeExternalServerStart() error`).

## Out of scope

- Changes to minification logic, icon registration, or SSR module extraction.
- The internal-mode HTTP serving path ([assetmin/http.go](../http.go)) — it remains
  unchanged; it just becomes irrelevant once the external server takes over.
- The `tinywasm/client` storage API rename (`SetBuildOnDisk` → `UseDiskStorage` /
  `UseMemoryStorage`) is owned by [client PLAN](../../client/docs/PLAN.md) —
  coordinated with this PR, not a follow-up.

## Acceptance criteria

1. `SetExternalSSRCompiler` and `SetBuildOnDisk` no longer exist in the public API.
2. `FileWriteSafe` is removed from `filewrite.go`.
3. `EnableSSRMode`, `SetSSRCompiler` (pure setter), and `FlushToDisk` exist with the
   signatures defined in §1.
4. `FlushToDisk` writes every asset in `c.allAssets`, overwriting existing files,
   without holding `c.mu` during I/O (§5).
5. `c.diskMirrored` is set true **only** on full-success flush.
6. After a successful `FlushToDisk`, every subsequent in-memory mutation is mirrored
   to disk.
7. The full `go test ./...` suite under `assetmin/` passes.
8. The `app` integration (see app PLAN.md) compiles and the bug reproducer passes.
