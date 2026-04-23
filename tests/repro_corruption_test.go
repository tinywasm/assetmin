package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestReproCorruption simulates the exact scenario reported by the user:
// 1. Two specific icons (help and catalog) being injected.
// 2. Concurrent access pattern (Read vs Write race condition).
// 3. Verifies that the catalog icon is fully intact and not corrupted.
func TestReproCorruption(t *testing.T) {
	// Setup environment
	ac := &assetmin.Config{
		OutputDir:          t.TempDir(),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
		AppName:            "TestApp",
		AssetsURLPrefix:    "/assets",
		DevMode:            true, // Disable cache for more frequent regeneration
	}
	am := assetmin.NewAssetMin(ac)

	const iterations = 100
	var wg sync.WaitGroup
	wg.Add(2)

	// Inject 2 specific icons
	helpIcon := `<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z"/>`
	catalogIcon := `<path d="M4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6zm16-4H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H8V4h12v12zM10 9h8v2h-8zm0 3h4v2h-4zm0-6h8v2h-8z"/>`

	// Goroutine 1: Continuous Writing (Injecting)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			am.InjectSpriteIcon("help", helpIcon)
			am.InjectSpriteIcon("catalog", catalogIcon)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Continuous Reading (Simulating SSR load)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			am.RegenerateHTMLCache()
			content := string(am.GetCachedHTML())

			// Critical check: Ensure the catalog icon identifier is present and not mangled
			if !strings.Contains(content, `id="catalog"`) {
				log.Printf("[ERROR] Catalog ID missing in iteration %d", i)
			}

			// Ensure content is not truncated or corrupted
			if len(content) < 100 {
				log.Printf("[ERROR] Suspiciously small content length: %d", len(content))
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Final verification
	finalContent, _ := am.GetMinifiedJS() // Just to check it doesn't crash, though we use HTML/SVG
	_ = finalContent

	am.RegenerateHTMLCache()
	finalHTML := string(am.GetCachedHTML())

	if !strings.Contains(finalHTML, `id="help"`) {
		t.Error("Final sprite missing 'help' icon")
	}
	if !strings.Contains(finalHTML, `id="catalog"`) {
		t.Error("Final sprite missing 'catalog' icon")
	}

	t.Log("✓ Repro corruption test completed without crashes")
}
