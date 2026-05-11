# PLAN — Typed CSS migration for assetmin

## Goal

Drop the AST-based CSS extractor. assetmin obtains a component's CSS by invoking `RenderCSS()` / `RootCSS()` on the compiled module and calling `.String()` on the returned `*css.Stylesheet`.

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

Use the **`go run` indirection** pattern (no `plugin`, no CGO):

1. assetmin writes a tiny generated `main.go` to a temp dir:
   ```go
   package main
   import (
       "encoding/json"
       "os"
       target "<modulePath>"
   )
   func main() {
       inst := target.SSRInstance() // convention: each ssr.go exposes SSRInstance()
       out := map[string]string{
           "root":   inst.RootCSS().String(),
           "render": inst.RenderCSS().String(),
           "html":   inst.RenderHTML(),
           "js":     inst.RenderJS(),
       }
       icons := inst.IconSvg()
       json.NewEncoder(os.Stdout).Encode(struct{
           Strings map[string]string  `json:"strings"`
           Icons   map[string]string  `json:"icons"`
       }{out, icons})
   }
   ```
2. Run it with `go run` in the module's working directory; capture stdout.
3. Unmarshal into `SSRAssets`.

**Convention added:** every component's `ssr.go` exposes `func SSRInstance() <ProviderType>` returning a zero-value instance that implements the SSR interfaces. This avoids reflection and keeps the generated `main.go` trivial.

**Caching:** keyed by content hash of the module's Go files (excluding `*_test.go`). Skip the `go run` when unchanged. This keeps incremental SSR builds fast.

## Files removed

- `ssr_extract.go` (~160 lines)
- All AST helpers (`findEmbedVars`, `extractReturnString`, `evalStringExpr`, `extractReturnMap`).

## Files added

- `ssr_invoke.go` — generates the temp main, runs it, parses JSON.
- `ssr_cache.go` — content-hash cache of invoke results.
- `ssr_invoke_test.go` — fixture modules covering RootCSS, RenderCSS, RenderHTML, RenderJS, IconSvg.

## Files modified

- `ssr_register.go` — calls the new invoker instead of AST extractor.
- `tests/ssr_extract_root_test.go`, `tests/css_ssr_hotreload_test.go`, `tests/ssr_test.go`, `tests/ssr_refresh_test.go` — rewritten to exercise the invoke path. Fixtures must adopt the DSL.

## Steps

1. Define and document the `SSRInstance()` convention in `assetmin/docs/SSR.md`.
2. Implement `ssr_invoke.go` — generated `main.go` template + `go run` exec + JSON parse.
3. Implement `ssr_cache.go` — file-hash cache, invalidates on Go file mtime+size change.
4. Migrate one test fixture (button or a minimal stub) to the DSL + `SSRInstance()` and prove the invoker works end-to-end.
5. Migrate remaining fixtures.
6. Delete `ssr_extract.go` and AST helpers.
7. Run the full `tests/` suite; resolve regressions.
8. Update `docs/ARCHITECTURE.md` and `docs/SSR.md` to describe compile-and-invoke instead of AST parsing.

## Acceptance

- `ssr_extract.go` and its AST helpers no longer exist.
- All existing tests in `assetmin/tests/` pass against the new invoker.
- Cold extraction of a single component is ≤ 500 ms on a typical dev machine; cached extraction ≤ 5 ms.
- Hot-reload tests (`css_ssr_hotreload_test.go`, `ssr_refresh_test.go`) still pass — cache invalidation is correct on file change.

## Risks and mitigations

- **`go run` cost**: amortized by the content-hash cache. Worst case is the cold dev start; subsequent rebuilds touch only changed modules.
- **GOFLAGS / module resolution edge cases**: the invoker runs from `moduleDir` and inherits the parent process env. Test against vendored modules and multi-module workspaces.
- **Cross-platform path quoting**: use `os/exec` argv form, never string concatenation, when constructing the `go run` command.

## Out of scope

- Replacing `RenderJS` / `RenderHTML` with typed builders — same compile-and-invoke pipeline already supports them as strings; typed JS/HTML is a separate initiative.
- Plugin-based loading (`plugin.Open`) — rejected: not supported on all platforms, fragile across Go versions.

## Dependency

This plan depends on `tinywasm/css/docs/PLAN_typed_css.md` landing first: `*Stylesheet`, `.String()`, and the `SSRInstance()` convention must exist before assetmin can target them.
