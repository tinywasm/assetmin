# PLAN — Fix: Deep Subpackage SSR Load & Hot Reload

## Problem

Two bugs prevent CSS defined in nested sub-packages (e.g. `modules/contact/ssr.go`)
from being applied during the initial scan **and** on hot-reload.

Failing tests: `assetmin/tests/TestBug_DeepSubpackage_NotLoadedOnInitialScan`
               `assetmin/tests/TestBug_DeepSubpackage_HotReloadFails`

---

## Bug 1 — `moduleSubpackagesUsed` drops multi-segment sub-paths

**File:** `import_scanner.go:140`

```go
// Only support one level of subdirectories
if !strings.Contains(subPath, "/") {
```

When the import path is `example.com/demo/modules/contact`, the resolved
`subPath` is `"modules/contact"`, which contains `"/"` and is silently dropped.
Any `ssr.go` nested two or more levels below the module root is therefore never
extracted during `LoadSSRModules`.

### Fix

Remove the one-level constraint. Walk the full subPath to locate `ssr.go`:

```go
// Allow any depth — walk each path segment accumulating the dir.
if !seen[subPath] {
    usedSubpackages = append(usedSubpackages, subPath)
    seen[subPath] = true
}
```

Also update `loadSSRModulesLocked` in `ssr_loader.go`: when building the
sub-module dir use `filepath.Join(m.Dir, sub)` (already correct), but ensure
`extractSSRAssetsForModule` receives the deepest dir that actually contains
`ssr.go`, not the intermediate one.

---

## Bug 2 — `ExtractSSRAssets` requires `go.mod` in the exact `moduleDir`

**File:** `ssr_extract.go:28–31`

```go
if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
    return nil, fmt.Err("no go.mod found", err)
}
if _, err := os.Stat(filepath.Join(moduleDir, "ssr.go")); err != nil {
    return nil, fmt.Err("ssr.go not found in", moduleDir)
}
```

Sub-packages share the project root's `go.mod`. When the file-watcher fires
for `modules/contact/ssr.go`, `ReloadSSRModule` is called with `contactDir`,
which has `ssr.go` but no `go.mod`. The function returns immediately with
"no go.mod found" and the hot-reload is a no-op.

`findProjectRoot` (same file) already traverses parent directories to find
`go.mod` — but it is called **after** the early exit check, making it dead code
for this path.

### Fix

Replace the two `os.Stat` early exits with a single call to `findProjectRoot`:

```go
// Determine project root first — sub-packages share the root go.mod.
rootDir, err := findProjectRoot(moduleDir)
if err != nil {
    return nil, fmt.Err("failed to find project root from", moduleDir, err)
}

// ssr.go must exist in moduleDir (the sub-package), not at the root.
if _, err := os.Stat(filepath.Join(moduleDir, "ssr.go")); err != nil {
    return nil, fmt.Err("ssr.go not found in", moduleDir)
}
```

---

## Execution stages

| # | Task | File | Done |
|---|------|------|------|
| 1 | Remove one-level constraint in `moduleSubpackagesUsed` | `import_scanner.go` | [ ] |
| 2 | Reorder checks in `ExtractSSRAssets` — find root before stat go.mod | `ssr_extract.go` | [ ] |
| 3 | Pass tests: `TestBug_DeepSubpackage_NotLoadedOnInitialScan` | `tests/ssr_subpackage_deep_test.go` | [ ] |
| 4 | Pass tests: `TestBug_DeepSubpackage_HotReloadFails` | `tests/ssr_subpackage_deep_test.go` | [ ] |
| 5 | Run full test suite: `go test ./...` | all | [ ] |
| 6 | Verify live in goflare-demo via MCP browser screenshot | — | [ ] |
