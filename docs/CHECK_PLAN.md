# PLAN: Single-Winner `RootCSS` Replacement Semantics

## Context

`assetmin` discovers `RootCSS()` from two privileged sources:

- **`tinywasm/css`** — framework canonical tokens (always present).
- **Project root** — optional app-level theme.

Other modules that declare `RootCSS()` are ignored with a warning.

## Current behavior (to be replaced)

`resolveAndApplyRootCSS()` injects **both** blocks into the `open` slot: framework first, project second. CSS cascade resolves token conflicts.

```css
/* output */
:root { --color-background: var(--color-background-light); }     /* framework */
@media (prefers-color-scheme: dark) {
  :root { --color-background: var(--color-background-dark); }
}
:root { --color-background: #FAFAFA; }                            /* project */
```

### Why this is broken

1. **The `@media` trap.** If the project redeclares an *active* token outside of any media query, it silently breaks `prefers-color-scheme` switching for that token, because its unconditional `:root {}` lands after the framework's `@media` block in source order. The breakage is invisible — light mode still works.
2. **Redundancy.** Every token the framework declares appears in the output even when the project intends to ship its own complete theme.
3. **Implicit ownership.** Reading the output, it is not clear which declaration is authoritative for any given token.

## Target behavior

**Single-winner replacement.** When the project declares `RootCSS()`, it fully replaces the framework's contribution in the `open` slot. When it does not, the framework's `RootCSS()` is used as-is.

```go
// resolveAndApplyRootCSS, target shape
chosen := c.fromCss
if c.fromRoot != nil {
    chosen = c.fromRoot
}
// ... single ContentFile in contentOpen, or nil
```

Projects that want to inherit framework defaults must do so explicitly:

```go
// app root ssr.go
import "github.com/tinywasm/css"

func RootCSS() string {
    return css.RootCSS() + `
    :root {
      --color-primary:          #FF6B35;
      --color-background-light: #FAFAFA;
      --color-background-dark:  #121212;
    }`
}
```

This composition is explicit, testable, and gives the project total control over the order in which the cascade evaluates declarations.

## Contract recap

| Function | Slot | Content | Authoritative source |
|---|---|---|---|
| `RootCSS()` | `open` | `:root {}` tokens + `@media` for tokens | `tinywasm/css` OR app root (single winner) |
| `RenderCSS()` | `middle` | Component/feature rules that consume tokens via `var()` | Any module |

`RootCSS` and `RenderCSS` are mutually exclusive by intent:
- `RootCSS` declares variables; it does not style elements.
- `RenderCSS` styles elements; it does not declare global variables.

A module that needs both publishes both — `tinywasm/css` ships only `RootCSS`; `tinywasm/components/themeswitch` ships only `RenderCSS` (its `[data-theme]` blocks consume framework tokens).

## Execution steps

1. **Code change** — `assetmin/ssr_loader.go`:
   - Revert `resolveAndApplyRootCSS()` to single-winner: project beats framework, framework beats nothing.
   - `contentOpen` holds at most one `ContentFile`.

2. **Test updates**:
   - `tests/ssr_loader_root_override_test.go::TestLoader_AppAndCssBothInjected` → rename to `TestLoader_AppFullyReplacesCss`; assert only `--app:1` is present, `--css:1` is absent.
   - `tests/ssr_loader_reload_test.go::TestReload_AppGainsRootCSS` → restore "app fully replaces" expectation.
   - `TestReload_AppLosesRootCSS` already exercises the correct fall-back behavior; no change beyond variable naming already done.

3. **Documentation**:
   - Update `tinywasm/css/README.md` to describe replacement semantics and the explicit `css.RootCSS() + override` extension pattern.
   - Update doc comment on `css.RootCSS()` in `ssr.go` to match.
   - Drop language that implies "both are injected" or "cascade merges them".

4. **Migration note for consumers**: any app that currently relies on the merge behavior must either declare a self-contained `RootCSS()` or compose `css.RootCSS() + overrides`.

## Out of scope

- Helper such as `css.ExtendRootCSS(override string) string` — can be added later if the concat pattern proves common, but is not required for correctness.
- Multi-block stacking in `open` slot for unrelated open-slot content — keep the slot single-entry semantics intact.
