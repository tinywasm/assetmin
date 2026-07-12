//go:build !wasm

package assetmin_test

import (
	"testing"

	"github.com/tinywasm/assetmin"
)

// TestAssetRoutesArePublic pins the one property that decides whether anything
// renders at all: the router is PRIVATE BY DEFAULT — a route that never calls
// Public() answers 403 Forbidden to a caller with no identity. A browser
// fetching index.html, the stylesheet, the bundle or the favicon is always
// that caller. If these routes are not public, the page is a "Forbidden" body
// and nothing else, no matter how correct the build is.
func TestAssetRoutesArePublic(t *testing.T) {
	setup := newTestSetup(t)
	defer setup.cleanup()

	am := assetmin.NewAssetMin(setup.ac)

	if err := am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var a=1;"), "create"); err != nil {
		t.Fatalf("js event: %v", err)
	}
	if err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{}"), "create"); err != nil {
		t.Fatalf("css event: %v", err)
	}

	routes := newTestRouter(am).Routes()
	if len(routes) == 0 {
		t.Fatal("no routes registered")
	}

	for _, route := range routes {
		if !route.Public {
			t.Errorf("asset route %q is private → a browser gets 403 Forbidden; it must be .Public()", route.Path)
		}
	}
}
