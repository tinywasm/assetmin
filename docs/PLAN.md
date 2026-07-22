---
PLAN: "fix: deterministic SSR sprite — module-keyed icons + reliable mass scan"
STATUS: review
SESSION: 567306067129696064
PR: https://github.com/tinywasm/assetmin/pull/35
---

# PLAN — deterministic SVG sprite assembly

Self-contained fix plan for `github.com/tinywasm/assetmin`. Part of
`app-releases/docs/SVG_SPRITE_RUNTIME_MASTER_PLAN.md` (Phase A, primary). The
companion is `app/docs/PLAN_SPRITE_RUNTIME.md` (logger wiring + reload trigger).

**Regression tests MUST live in `tests/`** (package `assetmin_test`, black-box) —
every existing test is there. Use the injectable `SetSSRExtractor` seam and the
`ContainsSVG`/`HasIcon` observers (extend as noted).

## Problem (proven — see master plan §2)

`tinywasm/ssr` extraction is correct and stable (layout=6, components=2, total=8,
no drift). The served sprite loses non-root-subtree icons **non-deterministically**
because assetmin's runtime assembly has three defects:

1. **Icon accumulation is a blind append.** `mergeSprite` does
   `masterSprite.Merge(s)` and `sprite.String()` dedups nothing. Re-extraction on
   every hot reload appends duplicate `<symbol>`s (full set served twice observed);
   a browser uses the *first* symbol of an id, so an edited icon is shadowed by its
   stale earlier copy. CSS/JS never had this bug — they are stored **per module
   name** via `UpdateContentInSlot(name, …)`; only icons were left un-keyed.

2. **The background mass scan is best-effort and silent.** `ScheduleSSRLoad` runs
   `ExtractAll()` once; on error it calls `c.Logger(...)` — but `c.log` is nil in
   the daemon, so the failure vanishes. One transient `go run` failure (the
   extractor compiles a `main.go` while the WASM build contends on the build cache)
   permanently drops every non-root module's icons AND CSS until an unrelated save
   re-extracts them. No retry.

3. **The background merge does not refresh `.svg`.** Only `ReloadSSRModule`
   (the synchronous path) calls `refreshAsset(".svg")`; the background
   `ScheduleSSRLoad` path merges into `masterSprite` and invalidates the cache but
   never re-flushes, so a disk-served build keeps the stale sprite.

## Fix

### 1. Module-keyed, deduplicated sprite storage

Give icons the same per-module treatment CSS/JS already have, so re-extraction
REPLACES a module's icons instead of appending.

- Replace the single `masterSprite *sprite.Sprite` accumulator with
  `moduleSprites map[string]*sprite.Sprite` (guarded by `spriteMu`).
- `updateSSRModuleInSlot(name, …, icons, …)` — when `icons != nil`, do
  `c.setModuleSprite(name, icons)` which stores `c.moduleSprites[name] = icons`
  (replace, not append). When a module re-extracts with no icons, clear its entry.
- The manual `addIcon`/`addIconFile`/`InjectSpriteIcon` path keeps a dedicated
  bucket, e.g. `c.moduleSprites["_manual"]`, appended + dedup-checked as today.
- Add `func (c *AssetMin) renderSprite() string`: iterate `moduleSprites` in
  **sorted key order**, emit each icon **once per id** (first occurrence wins in
  that stable order), wrapped in the single `<svg aria-hidden…>…</svg>`. Route
  every current `masterSprite.String()` reader through it: the dynamic `.svg`
  handler (`assetmin.go`), `ContainsSVG`/`HasIcon` (`inspect.go`), and
  `checkIconID` (`svg.go`).

Result: idempotent accumulation, no duplicates, hot-reload edits replace in place.

### 2. Reliable mass scan (retry, loud, refresh)

In `ScheduleSSRLoad` (`ssr_loader.go`):

- Wrap `ExtractAll()` in a bounded retry: up to N attempts (e.g. 5) with backoff
  (e.g. 200ms → 3s), until `err == nil`. Each failed attempt is reported via
  `c.Logger("SSR ExtractAll attempt failed:", err)` — never swallowed.
- After the successful pass merges all modules, refresh the affected assets
  (`c.refreshAsset(".svg")`, `.css`, `.js`, `.html`) so a disk/flush-served build
  serves the complete result, then let the caller's reload hook re-serve.
- If all attempts fail, log a single loud terminal error (principle 6) — a broken
  extractor must be visible, not a page silently missing half its assets.

### 3. (covered by 2) background `.svg` refresh

Fold the `refreshAsset(".svg")` call into the successful mass-scan path above so
the background route matches the synchronous `ReloadSSRModule` route.

## Tests (`tests/`, black-box `assetmin_test`)

1. **Idempotent / no duplicates.** `UpdateSSRModule("mod", "", nil, "", spriteWithIcon("dup"))`
   twice → the served sprite contains `id="dup"` **exactly once**. `ContainsSVG` is
   boolean; add the count via the `.svg` handler output (`strings.Count(svg, "id=\"dup\"") == 1`).
2. **Cross-module union.** Update `"a"` with icon `ia` and `"b"` with icon `ib` →
   both present, each once, regardless of update order.
3. **Replace on re-extract.** Update `"mod"` with icon `old`, then again with icon
   `new` (no `old`) → served sprite has `new`, not `old` (module replace).
4. **Mass-scan retry (reliability).** Inject a fake `SSRExtractor` via
   `SetSSRExtractor` whose `ExtractAll` returns an error on the first call and the
   real assets on the second. After `LoadSSRModules()` + `WaitForSSRLoad`, the
   module's icons ARE served — proving a transient failure no longer permanently
   drops assets. (Before the fix: absent.)

## Verify

```
cd tinywasm/assetmin
go test ./tests/...     # new guards green
go test ./...
```

Then publish. Downstream, the daemon serves the full sprite deterministically
(master plan §5).

## Note

`SetLog` wiring is `app`'s responsibility (Phase B) — without it assetmin's new
loud diagnostics still reach a nil logger. Retry (fix 2) keeps correctness even
if wiring lags, but the two ship together for observability.
