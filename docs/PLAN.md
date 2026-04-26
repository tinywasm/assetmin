# PLAN: tinywasm/assetmin — Correcciones Pendientes

## Bug 1 — Data race entre `LoadSSRModules` y `WaitForSSRLoad`

### Síntoma
```
⚠️  WARNING: DATA RACE detected
runtime.racewrite  → assetmin.(*AssetMin).WaitForSSRLoad.func1()  ssr_loader.go:189
runtime.raceread   → assetmin.(*AssetMin).LoadSSRModules()         ssr_loader.go:26
```

### Causa raíz

`LoadSSRModules` hace `ssrLoading.Add(1)` **dentro** del goroutine del llamador.
Si el llamador lanza `LoadSSRModules` en un goroutine separado (como hace `app/section-build.go:170`)
y luego llama `WaitForSSRLoad()` antes de que ese goroutine llegue al `Add(1)`,
el `Wait()` interno de `WaitForSSRLoad` ve el contador en 0 y retorna inmediatamente,
mientras el otro goroutine aún está a punto de llamar `Add(1)` — data race en el WaitGroup.

### Corrección

Hacer `LoadSSRModules` internamente asíncrono: `Add(1)` ocurre **antes de retornar**,
así el llamador tiene garantizado el contador ≥ 1 cuando invoque `WaitForSSRLoad`.

```go
// ssr_loader.go — LoadSSRModules corregido (sin API nueva)
func (c *AssetMin) LoadSSRModules() {
    c.ssrLoading.Add(1)  // sincrónico: el llamador ya tiene el contador antes de Wait
    go func() {
        defer c.ssrLoading.Done()
        c.mu.Lock()
        defer c.mu.Unlock()
        c.loadSSRModulesLocked()  // lógica actual extraída sin Add/Done
    }()
}
```


### Test a añadir (`tests/ssr_loader_test.go`)

```go
// TestWaitForSSRLoadNoRace verifica que ScheduleSSRLoad+WaitForSSRLoad no producen
// data race cuando el goroutine interno aún no empezó al momento de llamar Wait.
func TestWaitForSSRLoadNoRace(t *testing.T) {
    root := t.TempDir()
    // módulo mínimo sin imports para que LoadSSRModules termine rápido
    os.WriteFile(filepath.Join(root, "go.mod"), []byte("module testrace\ngo 1.21\n"), 0644)

    c, _ := New(root)
    // Ejecutar múltiples veces para maximizar probabilidad de race en -race
    for range 20 {
        c.ScheduleSSRLoad()
        c.WaitForSSRLoad(2 * time.Second)
    }
}
```

Correr con: `go test -race ./tests/ -run TestWaitForSSRLoadNoRace`

---

## Orden de ejecución

| # | Tarea | Archivo | Estado |
|---|-------|---------|--------|
| 1 | Extraer `loadSSRModulesLocked`, hacer `LoadSSRModules` async internamente | `ssr_loader.go` | Pendiente |
| 2 | Añadir `TestWaitForSSRLoadNoRace` con `-race` | `tests/ssr_loader_test.go` | Pendiente |
| 3 | Publicar nueva versión de `tinywasm/assetmin` | — | Pendiente (después de 1 y 2) |
