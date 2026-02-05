package assetmin

import (
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
	ac := &Config{
		OutputDir:          t.TempDir(),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
		AppName:            "TestRepro",
		AssetsURLPrefix:    "/assets",
		DevMode:            true,
	}
	am := NewAssetMin(ac)
	am.SetLog(func(msg ...any) { log.Println(msg...) })

	// The exact icon strings from the user
	iconHelp := `<svg viewBox="0 0 16 16"><path fill="currentColor" fill-rule="evenodd" d="M16 8A8 8 0 1 1 0 8a8 8 0 0 1 16 0zM5.496 6.033h.825c.138 0 .248-.113.266-.25.09-.656.54-1.134 1.342-1.134.688 0 1.314.343 1.314 1.168 0 .635-.374.927-.965 1.371-.673.489-1.206 1.06-1.168 1.987l.003.217a.25.25 0 0 0 .25.246h.811a.25.25 0 0 0 .25-.25v-.105c0-.718.273-.927 1.01-1.486.609-.463 1.244-.977 1.244-2.056 0-1.511-1.276-2.241-2.726-2.241-1.385 0-2.439.728-2.536 2.193a.25.25 0 0 0 .249.257zM8 12.75c-.5 0-.917-.417-.917-.917 0-.5.417-.917.917-.917.5 0 .917.417.917.917 0 .5-.417.917-.917.917z"/></svg>`

	iconCatalog := `<path fill="currentColor" d="M11.5 0.5l-3.5 3 4.5 3 3.5-3z"></path>
<path fill="currentColor" d="M8 3.5l-3.5-3-4.5 3 3.5 3z"></path>
<path fill="currentColor" d="M12.5 6.5l3.5 3-4.5 2.5-3.5-3z"></path>
<path fill="currentColor" d="M8 9l-4.5-2.5-3.5 3 4.5 2.5z"></path>
<path fill="currentColor" d="M11.377 13.212l-3.377-2.895-3.377 2.895-2.123-1.179v1.467l5.5 2.5 5.5-2.5v-1.467z"></path>`

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Routine 1: Simulate server startup injecting icons (WRITER)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start // Wait for start signal

		// Inject continuously to trigger race
		for i := 0; i < 100; i++ {
			am.InjectSpriteIcon("icon-help", iconHelp)
			am.InjectSpriteIcon("catalog-module", iconCatalog)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Routine 2: Simulate browser requesting index.html (READER)
	// This triggers AddDynamicContent callback
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start

		for i := 0; i < 100; i++ {
			// Trigger HTML generation which reads the sprite
			am.indexHtmlHandler.RegenerateCache(am.min)

			// Verify content integrity immediately
			content := string(am.indexHtmlHandler.GetCachedMinified())
			if !strings.Contains(content, "catalog-module") {
				// It might be empty if not injected yet, thats fine
				continue
			}

			// The CRITICAL check: If it contains the ID, does it contain the FULL paths?
			// Specifically the last path of the catalog icon which was getting truncated
			if strings.Contains(content, "catalog-module") && !strings.Contains(content, "M11.377 13.212") {
				t.Errorf("CORRUPTION DETECTED: Sprite contains catalog-module ID but is missing the last path data (Truncated Read). Iteration %d", i)
			}

			time.Sleep(1 * time.Millisecond)
		}
	}()

	close(start) // START RACE
	wg.Wait()
}
