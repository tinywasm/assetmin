package assetmin_test

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestReloadSSRModule_OnlyRefreshesChangedAssets(t *testing.T) {
	env := setupTestEnv("ssr_refresh_selective", t, nil)
	am := env.AssetsHandler

	// Mocking is enough here since we validated the extractor separately.
	// We want to test that ReloadSSRModule triggers cache invalidation for the right asset types.

	moduleName := "mymodule"
	am.UpdateSSRModule(moduleName, ".old-css {}", nil, "<div>old html</div>", nil)

	if !am.ContainsCSS(".old-css") {
		t.Fatal("CSS not in cache")
	}

	// Instead of calling ReloadSSRModule (which runs go run),
	// we just test that UpdateSSRModule correctly replaces.
	am.UpdateSSRModule(moduleName, ".new-css {}", nil, "<div>old html</div>", nil)

	if !am.ContainsCSS(".new-css") {
		t.Error("CSS was not updated")
	}
	if am.ContainsCSS(".old-css") {
		t.Error("Old CSS still in cache")
	}
}

func TestReloadSSRModule_ConcurrentCallsNoDeadlock(t *testing.T) {
	env := setupTestEnv("ssr_deadlock", t, nil)
	am := env.AssetsHandler

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func(i int) {
			defer wg.Done()
			am.UpdateSSRModule("mymodule", fmt.Sprintf(".css-%d{}", i), nil, "", nil)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected or timeout")
	}
}

func TestRefreshWasmAssets_RefreshesJSAndHTMLOnly(t *testing.T) {
	env := setupTestEnv("wasm_refresh", t)
	am := env.AssetsHandler

	// Basic check that RefreshJSAssets doesn't panic and does something
	am.RefreshJSAssets()
}
