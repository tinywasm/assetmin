# PLAN — Fix empty `/style.css` for subpackage SSR modules

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

## Fix

The contract of `extractSSRAssetsForModule(m, rootDir, allModules, _)` is:
"return the SSR assets for `m`." It must therefore guarantee that `m`
appears in the module set passed to `invokeSSRExtractorOnce`.

Patch `extractSSRAssetsForModule`:

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

Also keep the stderr-capture fix in `invokeSSRExtractorOnce` so future
`go run` failures are visible in `c.Logger`-routed errors instead of
returning a bare `exit status 1`.

## Files to change

- `ssr_extract.go`:
  - `extractSSRAssetsForModule`: append `m` to the module set when missing
    before computing hash and invoking the extractor.
  - Add small helper `containsModule`.
- `ssr_invoke.go`: keep stderr-into-error wrap (already applied).
- `ssr_extract_subpackage_test.go`: stays as the regression test.

## Verification

1. `go test -run TestExtractSSRAssetsForModule_Subpackage -v .` — passes.
2. `go test ./...` in `assetmin/` — full suite stays green.
3. Restart tinywasm daemon, reload `layout/platformd/web` demo:
   - `curl -s http://localhost:6060/style.css | wc -c` returns ≫ 0.
   - `browser_evaluate_js` reports non-empty `getComputedStyle(.pd-root)`.

## Out of scope

- Fixing the rendering side-effect introduced by the recent `*Element` →
  `Element` change in `layout/platformd` (root tag is missing because
  `Render()` returns `&p.Element` with no tag set). That belongs in a
  PLAN under `layout/platformd/docs/`, not here.
- Auditing why `loadSSRModulesLocked` swallows extractor errors (`if err
  == nil`). Surfacing those errors to the logger is a follow-up — out of
  scope for this CSS-emptiness bug fix, but recommended.
