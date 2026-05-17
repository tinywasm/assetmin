# `FlushToDisk` — flow

Companion to [assetmin/docs/PLAN.md](../PLAN.md). Cross-package view lives in
[app/docs/diagrams/EXTERNAL_MODE_TRANSITION.md](../../../app/docs/diagrams/EXTERNAL_MODE_TRANSITION.md).

## Current (buggy) flow — `SetExternalSSRCompiler(_, true)`

```mermaid
flowchart TD
    Start([caller: SetExternalSSRCompiler fn, true]) --> Lock[mu.Lock]
    Lock --> Save[c.onSSRCompile = fn<br/>c.buildOnDisk = true]
    Save --> CallFn[fn  -- often noop --]
    CallFn --> Loop{for each of<br/>5 hardcoded handlers}
    Loop --> Regen[asset.RegenerateCache]
    Regen --> Check{outputPath<br/>exists on disk?}
    Check -- yes --> Skip[FileWriteSafe returns nil ❌<br/>stale bytes survive]
    Check -- no --> Write[FileWrite outputPath]
    Skip --> Loop
    Write --> Loop
    Loop --> Unlock[mu.Unlock — holds during all I/O ❌]
    Unlock --> End([return — no error reported])

    style Skip fill:#fdd,stroke:#900
    style Loop fill:#ffd,stroke:#990
    style Unlock fill:#fdd,stroke:#900
```

Defects:
- **B1** the `Skip` branch dominates in a dev session (files exist).
- **B2** loop iterates only 5 main handlers.
- **B3** API conflates SSR-mode flag, compiler registration, and disk flush.
- Lock held during all disk I/O.

## Expected (post-fix) flow — `FlushToDisk()` with snapshot-then-write

```mermaid
flowchart TD
    Start([caller: FlushToDisk]) --> Lock[mu.Lock]
    Lock --> Loop1{for each asset in<br/>c.allAssets map}
    Loop1 --> Regen[asset.RegenerateCache]
    Regen --> Snap[append outputPath + bytes<br/>to local snapshot slice]
    Snap --> Loop1
    Loop1 -- done --> Unlock[mu.Unlock ✅<br/>readers unblocked]
    Unlock --> Loop2{for each in snapshot}
    Loop2 --> Write[FileWrite outputPath<br/>OVERWRITE ✅]
    Write --> Err{write err?}
    Err -- yes --> CaptureErr[firstErr = err if nil]
    Err -- no --> Loop2
    CaptureErr --> Loop2
    Loop2 -- done --> Check{firstErr == nil?}
    Check -- yes --> Mark[mu.Lock<br/>c.diskMirrored = true<br/>mu.Unlock]
    Check -- no --> NoMark[diskMirrored stays false ✅]
    Mark --> RetOK([return nil])
    NoMark --> RetErr([return firstErr])

    style Write fill:#dfd,stroke:#090
    style Mark fill:#dfd,stroke:#090
    style NoMark fill:#dfd,stroke:#090
    style Unlock fill:#dfd,stroke:#090
```

Key guarantees:
- ✅ `c.mu` is NOT held during disk I/O (no blocking of HTTP serving / watcher events).
- ✅ `c.diskMirrored` is set **only** on a fully successful flush.
- ✅ Overwrite, never `FileWriteSafe`.
- ✅ Asset set is `c.allAssets` (deduplicated map keyed by `outputPath`).

After a successful `FlushToDisk`, the per-event path mirrors to disk:

```mermaid
flowchart LR
    Evt[file event<br/>NewFileEvent] --> Update[UpdateFileContentInMemory]
    Update --> Process[processAsset]
    Process --> Cache[regenerate cache]
    Cache --> Mirror{c.diskMirrored?}
    Mirror -- no --> Done([in-memory only])
    Mirror -- yes --> Disk[FileWrite outputPath ✅] --> Done2([disk + memory in sync])
```

## State decoupling (B3 fix)

```mermaid
flowchart LR
    subgraph Before[BEFORE — conflated]
        B1[isSSRMode is c.onSSRCompile != nil]
        B2[buildOnDisk bool]
        B3[SetExternalSSRCompiler<br/>does 4 things at once]
    end
    subgraph After[AFTER — orthogonal]
        A1[c.ssrEnabled bool] --- E1[EnableSSRMode]
        A2[c.onSSRCompile func] --- E2[SetSSRCompiler — pure setter]
        A3[c.diskMirrored bool] --- E3[FlushToDisk — set on success]
    end
```

`isSSRMode()` returns `c.ssrEnabled` (no longer inferred from compiler nilness).
`SetSSRCompiler` is a **pure setter** — it does NOT auto-invoke the registered function.

## Asset enumeration (B2 fix)

```mermaid
flowchart TD
    Ctor[NewAssetMin] --> Reg5[register the 5 main handlers<br/>→ c.allAssets map]
    Ev1[UpdateFileContentInMemory<br/>new module CSS] --> Add[c.allAssets outputPath = asset<br/>dedupes by outputPath]
    Ev2[addIcon — extra SVG] --> Add
    Ev3[any future per-module asset] --> Add
    Add --> Enum[FlushToDisk iterates<br/>c.allAssets map — N unique, not 5 ✅]
```

## API surface change

| Before                                          | After                                     |
|-------------------------------------------------|-------------------------------------------|
| `SetExternalSSRCompiler(fn, buildOnDisk)`       | split into 3 single-responsibility APIs   |
| `SetBuildOnDisk(bool)` (deprecated)             | `FlushToDisk() error`                     |
| `isSSRMode = (onSSRCompile != nil)`             | `EnableSSRMode()` sets `c.ssrEnabled`     |
| compiler auto-invoked on registration           | `SetSSRCompiler(fn)` is a pure setter     |
| `buildOnDisk` field                             | `diskMirrored` (set by successful flush)  |
| `FileWriteSafe`                                 | removed; use `FileWrite`                  |
| asset set = 5 hardcoded handlers                | `c.allAssets map[string]*asset`           |
| lock held during I/O                            | snapshot-then-write (unlock before I/O)   |
