# Event Flow & Hot-Reload Sequence

This sequence diagram explains how `AssetMin` processes file events during development. It specifically highlights the dual-path SSR hot-reloading mechanism, which avoids expensive WASM rebuilds when editing embedded assets.

```mermaid
sequenceDiagram
    participant Watcher as File Watcher
    participant Events as AssetMin (events.go)
    participant Loader as SSR Loader / AST
    participant WASM as External Compiler (WASM)
    participant Cache as Asset Cache

    Watcher->>Events: NewFileEvent(filePath, extension)
    
    alt is SSR Mode == false
        Events->>Cache: UpdateFileContentInMemory()
        Cache-->>Events: Content Updated
        Events->>Cache: RegenerateCache()
        
    else is SSR Mode == true
    
        alt extension == ".go"
            Events->>WASM: onSSRCompile()
            WASM-->>Events: Full WASM Rebuild (Slow)
            
        else extension in [".css", ".js", ".html", ".svg"]
            Events->>Loader: ReloadSSRModule(dir)
            
            alt module exists in dir
                Loader->>Loader: Parse ssr.go & read //go:embed
                Loader->>Cache: UpdateSSRModuleInSlot()
                Loader-->>Events: Success
                Events->>Cache: RefreshAsset(extension)
                Note over Events,Cache: Fast Hot-Reload (No WASM build)
            else no ssr.go
                Loader-->>Events: Error (Not an SSR module)
                Note over Events: Ignore loose file to avoid duplication
            end
            
        else other extensions (.md, .json, etc.)
            Events-->>Watcher: Return nil (Ignored)
        end
    end
```
