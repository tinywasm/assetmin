# assetmin — Plan: Route `RootCSS()` to the `open` slot, drop dom hardcoding

## Problem

`assetmin/ssr_loader.go` currently decides the slot of an extracted module via:

```go
slot := "middle"
if strings.Contains(m.Path, "tinywasm/dom") { slot = "open" } else if isRootDir(...) { slot = "close" }
```

Three issues:

1. **Hardcoded module name**. `assetmin` should not know about `tinywasm/dom`. The coupling is supposed to flow one way: assetmin scans, dom exposes a convention.
2. **No way to ship root-level CSS** (the `:root { … }` token block) without being literally named `tinywasm/dom`. A third-party theme module or the app project itself cannot place CSS in `open`.
3. **Static extraction is silent on intent**. Today the slot is inferred from the module path, not from what the module actually declares. If `tinywasm/dom`'s `RenderCSS` returns an unparseable expression — for example `func (c CssVars) RenderCSS() string { return c.renderCSS() }`, where the return is an `*ast.CallExpr` the extractor cannot evaluate — the slot is still `open` but the extracted CSS is the empty string. Nothing tells the user something went wrong.

## Decision

Introduce a name-based contract:

| Static function in `ssr.go` | Extractor field | Slot routing |
|---|---|---|
| `RootCSS() string` | `SSRAssets.RootCSS` | `open` (single override, see below) |
| `RenderCSS() string` | `SSRAssets.CSS` | `close` if root project, else `middle` |
| `RenderJS() string` | `SSRAssets.JS` | same as CSS |
| `RenderHTML() string` | `SSRAssets.HTML` | same as CSS |
| `IconSvg() map[string]string` | `SSRAssets.Icons` | icon registry (no slot) |

**Single-override rule for `RootCSS()`** — at most one entry occupies the `open` slot at any time:

- If the **root project** declares `RootCSS()` → it wins; dom's is ignored.
- Otherwise, if **`tinywasm/dom`** declares `RootCSS()` → that one is used (default fallback).
- If a **third-party module** (neither root nor dom) declares `RootCSS()` → ignored, and a warning is logged via `c.Logger`. Reason: `:root` is a global namespace; only the app or the documented theme provider may write it. Letting any transitive dependency contribute would silently corrupt the theme.

dom is the only module the loader names. That coupling is one-way and loose: dom does not import assetmin, and assetmin only references dom by string-matching the module path to recognize it as the *fallback* theme provider. Replacing dom with another fallback in the future means changing one constant in assetmin.

## Changes

### 1. `assetmin/ssr_extract.go` — add `RootCSS` field and extraction

Add field:

```go
type SSRAssets struct {
    ModuleName string
    RootCSS    string  // NEW — populated from func RootCSS() string
    CSS        string
    JS         string
    HTML       string
    Icons      map[string]string
}
```

In the `ast.Inspect` switch on `fn.Name.Name`, add:

```go
case "RootCSS":
    assets.RootCSS = extractReturnString(fn, embeds, moduleDir)
```

No changes needed to `extractReturnString` / `evalStringExpr` — `RootCSS()` is expected to return either a `//go:embed` var (an `*ast.Ident`), a string literal, or a concatenation, all already supported.

### 2. `assetmin/ssr_loader.go` — replace slot resolution and apply override

Define the dom fallback identifier in one place:

```go
// domModulePath is the module path that provides the default `:root` theme
// when the root project does not declare its own RootCSS().
const domModulePath = "tinywasm/dom"
```

Rewrite `loadSSRModulesLocked` so the loop collects assets and resolves the root-CSS override in a single pass:

```go
type rootCandidate struct {
    name string
    css  string
}

var fromRoot, fromDom *rootCandidate

for _, m := range modules {
    if m.Dir == "" { continue }
    
    isDom  := strings.Contains(m.Path, domModulePath)
    isRoot := isRootDir(m.Dir, c.RootDir)
    alwaysLoad := isDom || isRoot
    
    if alwaysLoad {
        if assets, err := ExtractSSRAssets(m.Dir); err == nil {
            c.routeAssets(assets, isRoot, isDom, &fromRoot, &fromDom)
        }
    }
    
    for _, sub := range moduleSubpackagesUsed(m.Path, m.Dir, importedPaths) {
        if sub == "" && alwaysLoad { continue }
        subDir := filepath.Join(m.Dir, sub)
        if assets, err := ExtractSSRAssets(subDir); err == nil {
            subIsDom  := strings.Contains(subDir, domModulePath)
            subIsRoot := isRootDir(subDir, c.RootDir)
            c.routeAssets(assets, subIsRoot, subIsDom, &fromRoot, &fromDom)
        }
    }
}

// Resolve single-override for the open slot.
chosen := fromDom
if fromRoot != nil { chosen = fromRoot }
if chosen != nil {
    c.updateSSRModuleInSlot(chosen.name, chosen.css, "", "", nil, "open")
}
```

`routeAssets` (new private method) handles the per-module routing:

```go
func (c *AssetMin) routeAssets(a *SSRAssets, isRoot, isDom bool, fromRoot, fromDom **rootCandidate) {
    if a.RootCSS != "" {
        switch {
        case isRoot:
            *fromRoot = &rootCandidate{name: a.ModuleName, css: a.RootCSS}
        case isDom:
            *fromDom = &rootCandidate{name: a.ModuleName, css: a.RootCSS}
        default:
            c.Logger("warning: module", a.ModuleName, "declares RootCSS() but only the root project or", domModulePath, "may; ignoring")
        }
    }
    
    slot := "middle"
    if isRoot { slot = "close" }
    // RootCSS deliberately NOT passed here — it has its own slot resolution above.
    c.updateSSRModuleInSlot(a.ModuleName, a.CSS, a.JS, a.HTML, a.Icons, slot)
}
```

Apply the same single-override resolution in `ReloadSSRModule`. When a hot reload re-extracts an `ssr.go`, the loader must:

- If the reloaded module is dom or root → re-evaluate which `RootCSS` wins and re-write the `open` slot.
- If a third-party module is reloaded and now declares `RootCSS()` → log warning, do not write to `open`.

Concretely, factor the override resolution out of `loadSSRModulesLocked` into a helper `resolveAndApplyRootCSS()` that both call paths use. Cache `fromRoot` / `fromDom` on `*AssetMin` so reloads see the persistent state, not just the current call's local vars.

### 3. `assetmin/ssr_register.go` — runtime root-CSS provider

Add the symmetric runtime interface:

```go
type rootCssProvider interface{ RootCSS() string }
```

In `RegisterComponents`, when a provider implements it, route through the same single-override gate (treat the registration as coming from the root project — runtime registration is always under app control).

```go
if rp, ok := p.(rootCssProvider); ok {
    rootCSS := rp.RootCSS()
    if rootCSS != "" {
        c.UpdateSSRModuleInSlot(fmt.Sprintf("%T", p), rootCSS, "", "", nil, "open")
    }
}
```

`UpdateSSRModuleInSlot` is already idempotent on `name`, so a later registration with the same name replaces the earlier content. To keep the single-override invariant when several runtime providers register, emit a warning if `open` is already populated by a different name. (This is a soft invariant; the slot system technically allows two entries, but the documented contract is one.)

### 4. `assetmin/docs/SSR.md` — update the contract

Replace the "Asset Declaration" section with the new function table (copy from this plan). Replace the "Module Order" subsection with:

```
- Root-level CSS (`open` slot): the single winner of the override resolution
  (root project's RootCSS, else tinywasm/dom's RootCSS).
- Module CSS / JS / HTML / Icons: from each module's RenderCSS / RenderJS /
  RenderHTML / IconSvg, injected into the `middle` slot for dependencies and
  the `close` slot for the root project.
```

Document the third-party warning. Document that `RenderCSS` is no longer how dom contributes its theme — it is now reserved for component-level CSS.

## Order of implementation

1. `ssr_extract.go`: add `RootCSS` field + `case "RootCSS"` branch. Keep everything else untouched.
2. New extractor unit tests (see below).
3. `ssr_loader.go`: introduce `domModulePath`, `rootCandidate`, `routeAssets`, `resolveAndApplyRootCSS`. Remove all `strings.Contains(..., "tinywasm/dom")` *for slot decisions* — the only remaining reference is in `routeAssets` distinguishing the dom-fallback case.
4. New loader unit tests (see below).
5. `ssr_register.go`: add `rootCssProvider`, route through `open`.
6. New register unit tests (see below).
7. `docs/SSR.md`: update.
8. `go test ./...` from `assetmin/` clean.

## Tests

All under `assetmin/tests/` following the existing pattern. Each test must use the in-process module discovery override (`c.listModulesFn`) — no real `go list` calls.

### `tests/ssr_extract_root_test.go` (new)

| Test | Fixture (synthetic `ssr.go`) | Assertion |
|---|---|---|
| `TestExtract_RootCSS_FromEmbed` | `//go:embed theme.css` + `func RootCSS() string { return rootCSS }` and a real `theme.css` next to it | `assets.RootCSS` equals the file contents |
| `TestExtract_RootCSS_FromLiteral` | `func RootCSS() string { return ":root{--x:1;}" }` | `assets.RootCSS == ":root{--x:1;}"` |
| `TestExtract_RootCSS_FromConcat` | `const a = ":root{"; func RootCSS() string { return a + "}" }` (only literal+literal supported) → use `func RootCSS() string { return ":root{" + "}" }` | `assets.RootCSS == ":root{}"` |
| `TestExtract_RootCSS_Missing` | `ssr.go` with no `RootCSS` declaration | `assets.RootCSS == ""`, no error |
| `TestExtract_BothRootAndRender` | `ssr.go` with both `RootCSS()` and `RenderCSS()` returning different strings | both `RootCSS` and `CSS` fields populated independently |
| `TestExtract_RootCSS_UnparseableExpr` | `func RootCSS() string { return computeIt() }` (a `*ast.CallExpr`) | `assets.RootCSS == ""`, no panic, no error (silent fallback — assetmin reports zero-content via downstream, not here) |

### `tests/ssr_loader_root_override_test.go` (new)

Each fixture sets up a fake project tree with a `go.mod`, a fake dom module (in a `replace`-style local dir), and zero-or-more third-party modules.

| Test | Modules declaring `RootCSS()` | Expected `open` slot content | Expected log |
|---|---|---|---|
| `TestLoader_DomDefaultWins_NoAppRoot` | dom only | dom's `RootCSS` | (none) |
| `TestLoader_AppOverridesDom` | dom + root project | root project's `RootCSS` | (none) |
| `TestLoader_AppRootCSS_DomMissing` | root project only (dom has no `ssr.go`) | root project's `RootCSS` | (none) |
| `TestLoader_NoneDeclareRoot` | nothing declares it | `open` slot empty | (none) |
| `TestLoader_ThirdPartyIgnored` | dom + third-party | dom's `RootCSS` | warning containing `"declares RootCSS()"` and the third-party module name |
| `TestLoader_ThirdPartyIgnored_NoDom` | third-party only | `open` slot empty | warning |
| `TestLoader_TwoThirdParties_BothIgnored` | two third-parties | `open` slot empty | two warnings |
| `TestLoader_RegularCSSStillRoutes` | dom (`RootCSS`) + third-party (`RenderCSS`) + root (`RenderCSS`) | `open` = dom's RootCSS; `middle` contains third-party CSS; `close` contains root project CSS | (none) |
| `TestLoader_ModuleDeclaresBoth` | a third-party module declares `RootCSS()` AND `RenderCSS()` (third-party `RootCSS` is ignored, but `RenderCSS` still routes) | `open` = dom's RootCSS (or empty if dom absent); `middle` contains the third-party's `RenderCSS` content | warning for the ignored RootCSS |
| `TestLoader_NoHardcodedDomInSlot` | use a `tinywasm/foo` module that declares `RootCSS()` (acting as if the dev forked dom under another name) | `open` slot empty (foo is treated as third-party); warning logged | warning |

The last test explicitly proves that `open` is gated by **`tinywasm/dom` path match**, not by any other heuristic. If we ever decide to make the fallback module configurable, this is the test that documents the current default.

### `tests/ssr_loader_reload_test.go` (new)

| Test | Sequence | Assertion |
|---|---|---|
| `TestReload_AppGainsRootCSS` | Initial: dom only. Then root project's `ssr.go` is rewritten to include `RootCSS()`. Call `ReloadSSRModule(rootDir)`. | `open` slot now contains the app's RootCSS; dom's is no longer present. |
| `TestReload_AppLosesRootCSS` | Initial: root project has `RootCSS()`. Rewrite `ssr.go` to remove it. Reload. | `open` slot falls back to dom's RootCSS. |
| `TestReload_ThirdPartyAddsRootCSS` | Initial: dom only. Third-party gains `RootCSS()`. Reload. | `open` unchanged (still dom's). Warning logged. |
| `TestReload_DomChangesRootCSS` | Initial: dom only. Rewrite dom's `theme.css`. Reload. | `open` slot reflects new dom content. |

### `tests/ssr_register_root_test.go` (new)

| Test | Provider | Assertion |
|---|---|---|
| `TestRegister_RootCssProvider_Empty` | type implements `RootCSS()` returning `""` | `open` slot not written |
| `TestRegister_RootCssProvider_NonEmpty` | type implements `RootCSS()` returning `:root{--a:1;}` | `open` slot contains `:root{--a:1;}` keyed under `fmt.Sprintf("%T", p)` |
| `TestRegister_RootCssOverridesPrevious` | register provider A, then provider B with same `%T` key | `open` slot contains B's content (replacement) |
| `TestRegister_RootAndCssProvider` | type implements both `RootCSS()` and `RenderCSS()` | `open` gets RootCSS, `middle` gets RenderCSS, both keyed by the same `%T` |

### `tests/ssr_extract_test.go` (existing — verify still passes unchanged)

The existing extractor tests must keep passing without modification. Adding `RootCSS` is purely additive on `SSRAssets`.

## Coverage matrix (every case enumerated above)

```
Where can RootCSS() come from?
├─ dom only          → loader_root_override: TestLoader_DomDefaultWins_NoAppRoot, TestReload_DomChangesRootCSS
├─ root project only → loader_root_override: TestLoader_AppRootCSS_DomMissing,    TestReload_AppGainsRootCSS, TestReload_AppLosesRootCSS
├─ both              → loader_root_override: TestLoader_AppOverridesDom
├─ third-party       → loader_root_override: TestLoader_ThirdPartyIgnored, TestLoader_ThirdPartyIgnored_NoDom, TestLoader_TwoThirdParties_BothIgnored, TestReload_ThirdPartyAddsRootCSS
├─ nobody            → loader_root_override: TestLoader_NoneDeclareRoot
├─ mis-named module  → loader_root_override: TestLoader_NoHardcodedDomInSlot
└─ runtime register  → register_root: all 4 cases

How is RootCSS() expressed in source?
├─ //go:embed var → ssr_extract_root: TestExtract_RootCSS_FromEmbed
├─ string literal → ssr_extract_root: TestExtract_RootCSS_FromLiteral
├─ concatenation  → ssr_extract_root: TestExtract_RootCSS_FromConcat
├─ unsupported    → ssr_extract_root: TestExtract_RootCSS_UnparseableExpr
└─ absent         → ssr_extract_root: TestExtract_RootCSS_Missing

Coexistence with other assets?
└─ both RootCSS and RenderCSS in same module → ssr_extract_root: TestExtract_BothRootAndRender, loader_root_override: TestLoader_DomDeclaresBoth, TestLoader_RegularCSSStillRoutes
```

## Breaking changes

| Change | Impact | Mitigation |
|---|---|---|
| `SSRAssets.RootCSS` field added | Additive — existing consumers unaffected | None |
| Loader no longer routes any module to `open` based on path alone | A module previously named `tinywasm/dom` that returned non-empty `RenderCSS()` was placed in `open`; now it goes to `middle` | dom's `RenderCSS()` was always broken (returned empty for embedded content via `*ast.CallExpr`). No live consumer exists. |
| Third-party `RootCSS()` ignored | If anyone shipped a `RootCSS()` expecting it to land somewhere, it silently does not | Prior to this plan no `RootCSS()` convention existed — there are no such consumers |
| `domModulePath` constant | New symbol in `ssr_loader.go`, unexported | None |

## Out of scope

- Whether `domModulePath` should become configurable via `Config` to support alternate fallback theme providers. Defer until a second fallback is requested. Today the rule is hard: `tinywasm/dom` or nothing.
- Multi-tenant theming (different `:root` per page). The `open` slot is a single document-level value.
