# CHECK_PLAN — Fix empty `/style.css` for subpackage SSR modules

> **Status:** code patches applied. Unit test green. Production browser
> verification still pending (see _Verification_ below).

## Problem

In production (tinywasm/app serving the `layout/platformd` demo), the
browser receives `/style.css` with 0 bytes even though
`platformd.SSRInstance().RenderCSS().String()` produces ~6.6 KB of valid CSS.

A reproducer test lives at `ssr_extract_subpackage_test.go`:

```
$ go test -run TestExtractSSRAssetsForModule_Subpackage -v .
--- FAIL: TestExtractSSRAssetsForModule_Subpackage
    extractSSRAssetsForModule returned error: go run failed exit status 1
    /tmp/assetmin-extract-*/main.go:7:2: "example.com/parent" imported and not used
```

## Root Cause

`loadSSRModulesLocked` (in `ssr_loader.go`) iterates the dependency modules
returned by `go list -m -json all`, then for each module it walks discovered
**subpackages** and synthesizes a `Module` value:

```go
subM := Module{Path: m.Path, Dir: subDir}
if sub != "" {
    subM.Path = m.Path + "/" + sub
}
if assets, err := extractSSRAssetsForModule(subM, c.RootDir, modules, ""); err == nil {
    ...
}
```

The synthesized `subM` is **not** appended to `modules` before being passed
to `extractSSRAssetsForModule`.

Inside `extractSSRAssetsForModule` (`ssr_extract.go`):

1. The hash cache key is computed from `allModules` (the parent list,
   without the subpackage).
2. `invokeSSRExtractorOnce(rootDir, allModules)` generates a temp `main.go`
   that imports only the modules in `allModules` and calls SSR funcs only
   on those that have `ssr.go`. The subpackage is silently absent.
3. Two failure modes follow:
   - If none of the parent modules have `ssr.go`, the generated `main.go`
     declares unused imports → `go run` fails → returns error → swallowed
     by the caller (`if err == nil`).
   - If at least one parent has `ssr.go`, `go run` succeeds, but the
     results map has no key for `subM.Path` → the function returns the
     zero-value fallback `&SSRAssets{ModuleName: filepath.Base(m.Dir)}` →
     `/style.css` stays empty.

The extra symptom (unused-import compile error) was masked by
`exec.Cmd.Output()` discarding stderr; this PLAN's first patch already
captures stderr into the wrapped error so future regressions surface.

## Fix (applied)

Two complementary patches landed:

### 1. `extractSSRAssetsForModule` ensures `m` is in the extractor set

The contract of `extractSSRAssetsForModule(m, rootDir, allModules, _)` is:
"return the SSR assets for `m`." It now guarantees that `m` appears in
the module set passed to `invokeSSRExtractorOnce`.

```go
func extractSSRAssetsForModule(m Module, rootDir string, allModules []Module, binCachePath string) (*SSRAssets, error) {
    // Ensure m is in the extractor's module set, so the generated main.go
    // imports it and the results map carries an entry for m.Path.
    modulesForExtract := allModules
    if !containsModule(allModules, m) {
        modulesForExtract = append(append([]Module(nil), allModules...), m)
    }

    hashKey, err := computeModuleHashSet(modulesForExtract)
    ...
    results, err := invokeSSRExtractorOnce(rootDir, modulesForExtract)
    ...
    output, ok := cachedResults[m.Path]
    ...
}

func containsModule(mods []Module, m Module) bool {
    for _, x := range mods {
        if x.Path == m.Path && x.Dir == m.Dir {
            return true
        }
    }
    return false
}
```

Behavioural notes:
- For top-level dep modules (already in `allModules`) nothing changes.
- For subpackages, `m` is appended → the generated `main.go` imports it →
  if it has `ssr.go` with `SSRInstance()`+`RenderCSS()`, those are called
  and the CSS reaches the results map.
- Hash key now varies per requested subpackage → cache entries are
  per-subpackage. That is the correct granularity (different subpackages
  may have different SSR outputs even when the parent module set is fixed).

### 2. `HasAnyFeature` gates imports in the generated `main.go`

`ssr_invoke.go` adds `ModuleAlias.HasAnyFeature()` and the template now
emits `import` and call blocks **only** for modules whose `ssr.go`
exposes at least one SSR function:

```gotmpl
{{if .HasAnyFeature}}{{.Alias}} "{{.Path}}"{{end}}
```

This eliminates the `imported and not used` compile failure that
manifested when a parent module without `ssr.go` was the only entry in
the module set. It is a defence-in-depth fix: the appended `m` from
patch (1) is what makes the subpackage's CSS reach the output, while
`HasAnyFeature` keeps the generated program compilable when the
non-SSR-bearing parent is also present.

### 3. Stderr capture in `invokeSSRExtractorOnce`

`exec.Cmd.Stderr` is now redirected into a buffer and folded into the
returned error, so future `go run` failures surface as
`go run failed: exit status 1: <compiler output>` rather than a bare
`exit status 1`. Without this, the original bug took longer to diagnose.

## Files changed

- [`ssr_extract.go`](../ssr_extract.go) — `containsModule` helper +
  `modulesForExtract` append in `extractSSRAssetsForModule`.
- [`ssr_invoke.go`](../ssr_invoke.go) — `HasAnyFeature()` method,
  template gates imports and call sites on it, stderr capture in
  `invokeSSRExtractorOnce`.
- [`ssr_extract_subpackage_test.go`](../ssr_extract_subpackage_test.go) —
  regression test, lives in `package assetmin` (internal) so it can
  exercise `extractSSRAssetsForModule` directly.

## Verification

| Step | Status |
|---|---|
| `go test -run TestExtractSSRAssetsForModule_Subpackage -v .` | ✅ PASS |
| `go test ./...` across `assetmin/`, `assetmin/tests/`, `assetmin/benchmark/` | ✅ PASS (~8s) |
| Restart tinywasm daemon, reload `layout/platformd/web` demo | ⏳ pending |
| `curl -s http://localhost:6060/style.css \| wc -c` returns ≫ 0 | ⏳ pending |
| `browser_evaluate_js` reports non-empty `getComputedStyle(.pd-root)` | ⏳ pending — also blocked on the platformd render bug (see [`layout/docs/PLAN.md`](../../layout/docs/PLAN.md)) |

## Out of scope

- The platformd `Render()` rendering bug (root `<` tag with no element
  name). Has its own plan at [`layout/docs/PLAN.md`](../../layout/docs/PLAN.md).
  That is a separate bug — even with this assetmin fix delivering CSS,
  the platformd HTML is malformed without that other patch.
- Auditing why `loadSSRModulesLocked` swallows extractor errors
  (`if err == nil` discards every error returned by
  `extractSSRAssetsForModule`). Surfacing those into `c.Logger` would
  have shortened diagnosis from hours to seconds. Recommended follow-up,
  but not required for this bug fix.
