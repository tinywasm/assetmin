# PLAN — Typed CSS migration for assetmin

## Goal

Replace the AST-based CSS extractor with a compile-and-invoke extractor. assetmin obtains every component's CSS by running a **single** generated `main.go` that imports all discovered components, calls their `RenderCSS()` / `RootCSS()`, and prints the aggregated `.String()` results as JSON to stdout. The extraction continues to happen — only the mechanism changes.

## Why

`ssr_extract.go` exists only because the current API returns `string` and assetmin must read the value without compiling the module. It supports exactly three authoring forms:
- `return "literal"`
- `return embedVar` (where `embedVar` carries a `//go:embed`)
- `return X + Y` (single binary `+`)

Anything else (helper calls, joins, slices, conditionals) silently produces `""`. As soon as components express CSS through a Go DSL (`tinywasm/css.New(Rule(...))`), the AST evaluator cannot follow the calls. Two options exist: extend the evaluator to track function calls across packages (impossible without a full Go interpreter), or compile and invoke the module (correct by construction).

The DSL change is what unlocks this simplification; this plan is the assetmin side of it.

## New API surface

`SSRAssets` keeps the same shape but the CSS fields are derived from typed values:

```go
type SSRAssets struct {
    ModuleName string
    RootCSS    string   // from (provider).RootCSS().String()
    CSS        string   // from (provider).RenderCSS().String()
    JS         string
    HTML       string
    Icons      map[string]string
}
```

`ExtractSSRAssets(moduleDir string) (*SSRAssets, error)` retains its signature but its implementation changes from AST traversal to compile-and-invoke.

## Implementation strategy

Use a **single combined `go run` invocation** for the whole component set (no `plugin`, no CGO, no per-module subprocesses).

Why combined: a per-module `go run` pays the Go compiler cold-start cost N times. For a typical app (~7 components) that is ~1.4 s of cold-start overhead. A single combined `main.go` compiles all imports once and emits aggregated JSON in ~300 ms — same robustness (the Go compiler still evaluates everything), ~5× faster cold path. Alternatives evaluated and rejected:
- **AST-walking DSL evaluator inside assetmin**: would duplicate the DSL semantics in assetmin, requiring synchronization on every property added to `tinywasm/css`. Same fragility class as the AST extractor it replaces.
- **In-process Go interpreter (yaegi)**: adds a megabyte-scale dependency and partial Go-spec coverage; marginal speed gain over option D.

### Generated extractor

1. assetmin discovers the set of component modules in the project (existing registration logic; no change to discovery).
2. assetmin writes one generated `main.go` to a temp dir importing **all** discovered components:
   ```go
   package main
   import (
       "encoding/json"
       "os"
       button "github.com/.../components/button"
       card   "github.com/.../components/card"
       // ... one alias per discovered component
   )
   type ssr struct {
       Root   string            `json:"root"`
       Render string            `json:"render"`
       HTML   string            `json:"html"`
       JS     string            `json:"js"`
       Icons  map[string]string `json:"icons"`
   }
   func collect(inst interface {
       RenderCSS() interface{ String() string }
       RenderHTML() string
       RenderJS() string
       IconSvg() map[string]string
   }, withRoot func() interface{ String() string }) ssr {
       out := ssr{
           Render: inst.RenderCSS().String(),
           HTML:   inst.RenderHTML(),
           JS:     inst.RenderJS(),
           Icons:  inst.IconSvg(),
       }
       if withRoot != nil { out.Root = withRoot().String() }
       return out
   }
   func main() {
       all := map[string]ssr{
           "button": collect(button.SSRInstance(), nil),
           "card":   collect(card.SSRInstance(),   nil),
           // ...
       }
       json.NewEncoder(os.Stdout).Encode(all)
   }
   ```
3. assetmin runs `go run main.go` **once**, captures stdout, unmarshals into `map[string]SSRAssets`.

**Convention added:** every component's `ssr.go` exposes `func SSRInstance() <ProviderType>` returning a zero-value instance that implements the SSR interfaces. This avoids reflection and keeps the generated `main.go` trivial.

**Caching:** keyed by content hash of every imported module's Go files (excluding `*_test.go`). The combined extractor is only regenerated when the hash set changes. Within a stable hash set, the previous JSON output is reused — `go run` is not invoked at all. This keeps incremental SSR builds at ~0 ms in steady state.

When a single module changes, the extractor is re-run (still a single invocation, still ~300 ms cold). Per-module isolation is not worth the per-module compilation overhead.

## Files removed

- `ssr_extract.go` (~160 lines) — **replaced by** `ssr_invoke.go`. The extraction step is not removed; only the AST-based implementation is.
- All AST helpers (`findEmbedVars`, `extractReturnString`, `evalStringExpr`, `extractReturnMap`) — no longer needed.

## Files added

- `ssr_invoke.go` — discovers components, generates the single combined `main.go`, runs `go run`, parses the aggregated JSON.
- `ssr_cache.go` — content-hash cache of the aggregated JSON output; one cache entry per (set-of-module-hashes).
- `ssr_invoke_test.go` — fixture covering RootCSS, RenderCSS, RenderHTML, RenderJS, IconSvg across multiple components in one extractor run.

## Files modified

- `ssr_register.go` — calls the new invoker instead of AST extractor.
- `events.go` — replace private `isSSRMode` reference with public `IsSSRMode`.
- `tests/ssr_extract_root_test.go`, `tests/css_ssr_hotreload_test.go`, `tests/ssr_test.go`, `tests/ssr_refresh_test.go` — rewritten to exercise the invoke path. Fixtures must adopt the DSL.

## Test fixture requirements

Every test that exercises `ExtractSSRAssets` or the combined invoker must provide a directory that satisfies `go run` / `go list`. Concretely, each stub module inside `t.TempDir()` must contain:

- `go.mod` with a valid module path (e.g. `module example.com/testcomp/button`) and a `go` directive matching the project's toolchain.
- At least one `.go` file with `package <name>` (the package name must match the last segment of the module path).
- A `func SSRInstance() *stubSSR` (or equivalent) that implements all four SSR interfaces: `RenderCSS()`, `RenderHTML()`, `RenderJS()`, `IconSvg()`.

Without `go.mod`, `go list` cannot resolve the import path and the combined extractor silently skips or errors the component. This is the root cause of tests finding fewer assets than expected.

For tests that previously used bare `.go` files (no module), add a helper `writeStubModule(t, dir, modulePath, pkgName string, body string)` that writes `go.mod` and the stub `.go` file in one call. All new and migrated fixtures must use this helper.

## Steps

1. Define and document the `SSRInstance()` convention in `assetmin/docs/SSR.md`.
2. Implement `ssr_invoke.go`: component discovery → combined `main.go` template → single `go run` exec → JSON parse → per-component `SSRAssets`.
3. Implement `ssr_cache.go`: hash-set cache, invalidates when any module's Go-file hashes change.
4. Fix `events.go`: replace `isSSRMode` (private, removed) with `IsSSRMode` (public).
5. Add `writeStubModule` test helper (internal to `tests/` package) and migrate one fixture set (two components) to it — prove combined invoker returns both in one run.
6. Migrate ALL remaining fixtures using `writeStubModule`; every `t.TempDir()` fixture must have `go.mod` + package + `SSRInstance()`.
7. Delete `ssr_extract.go` and AST helpers (replaced by `ssr_invoke.go`).
8. Run the full `tests/` suite; all tests must pass.
9. Update `docs/ARCHITECTURE.md` and `docs/SSR.md` to describe the single-invocation compile-and-invoke pipeline.

## Acceptance

- `ssr_extract.go` and its AST helpers no longer exist; `ssr_invoke.go` replaces them.
- All existing tests in `assetmin/tests/` pass against the new invoker.
- Cold extraction of a typical 7-component project is ≤ 500 ms on a dev machine (one combined `go run`); cached extraction ≤ 5 ms.
- Hot-reload tests (`css_ssr_hotreload_test.go`, `ssr_refresh_test.go`) still pass — cache invalidation is correct on file change.

## Risks and mitigations

- **`go run` cold cost**: ~300 ms for the combined extractor (single Go compile). Amortized by the hash-set cache in steady state. The combined form removes the N× multiplier of per-module invocation.
- **One bad component blocks all extraction**: a compilation error in any component fails the combined `go run`. Mitigation: surface the compiler error verbatim to the developer (same DX as `go build` in their own code) and skip cache write so the next run retries cleanly.
- **GOFLAGS / module resolution edge cases**: the invoker runs from the project root and inherits the parent process env. Test against vendored modules and multi-module workspaces.
- **Cross-platform path quoting**: use `os/exec` argv form, never string concatenation, when constructing the `go run` command.

## Out of scope

- Replacing `RenderJS` / `RenderHTML` with typed builders — same compile-and-invoke pipeline already supports them as strings; typed JS/HTML is a separate initiative.
- Plugin-based loading (`plugin.Open`) — rejected: not supported on all platforms, fragile across Go versions.

## Dependency

This plan builds on the typed CSS DSL foundation already published in `tinywasm/css` (`*Stylesheet`, `.String()`, tokens, `Class`, `Rule`, etc.). That dependency is satisfied — no further changes to `tinywasm/css` are required for this plan.

The `SSRInstance()` convention is **introduced by this plan**, not inherited from `tinywasm/css`. Components adopt it as part of their own migration (see `tinywasm/components/docs/PLAN.md`).

> The keyframes-only PLAN currently sitting at `tinywasm/css/docs/PLAN.md` is **not** a dependency of this plan — assetmin neither produces nor inspects `@keyframes`.
