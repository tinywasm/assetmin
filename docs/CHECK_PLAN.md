# AssetMin Improvement Plan

Three sequential stages addressing correctness, performance, and UX. Each stage must pass its own tests before the next begins.

## Stages

| # | File | Problem | Status |
|---|------|---------|--------|
| 1 | [PLAN_STAGE1_ssr_event_filter.md](PLAN_STAGE1_ssr_event_filter.md) | SSR mode must ignore all non-.go file events | TODO |
| 2 | [PLAN_STAGE2_selective_load.md](PLAN_STAGE2_selective_load.md) | Load ssr.go only from explicitly imported packages | TODO |
| 3 | [PLAN_STAGE3_minify_toggle.md](PLAN_STAGE3_minify_toggle.md) | Dynamic minification toggle via DevTUI HandlerExecution | TODO |

## Execution Rules

- Stages are **blocking**: do not start Stage N+1 until all Stage N tests pass.
- Each stage ships its own tests. Tests are the acceptance criteria.
- No stage modifies files owned by another stage.
- All code and comments must be in **English**.
- Log only errors or meaningful state changes — no noise.

## Affected Files Per Stage

```
Stage 1  → assetmin/events.go, assetmin/ssr.go, assetmin/docs/SSR.md
           assetmin/tests/ssr_event_filter_test.go (new)

Stage 2  → assetmin/ssr_loader.go, assetmin/import_scanner.go (new)
           assetmin/tests/selective_load_test.go (new)

Stage 3  → assetmin/minify_toggle.go (new)
           assetmin/tests/minify_toggle_test.go (new)
```
