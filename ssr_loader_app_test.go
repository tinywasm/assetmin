package assetmin

import (
	"strings"
	"testing"
)

// Bug adyacente del plan: routeAssets mandaba el módulo raíz al slot "close".
// Para el handler de HTML, contentClose arranca con </div> (y antes, con
// <div id="app"></div>), así que el RenderHTML() del raíz quedaba fuera de
// #app — para el shell viejo incluso después de </html>. Ahora el HTML
// ignora el slot: el "close" sigue siendo una decisión de cascada CSS/JS.
func TestRouteAssets_RootHTMLInsideApp(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir()})

	// Un módulo cualquiera primero, para fijar la posición de su CSS en middle.
	if err := c.routeAssets(&SSRAssets{
		ModuleName: "example.com/mod",
		CSS:        ".mod{color:blue}",
		HTML:       `<section>Module</section>`,
	}, false, false); err != nil {
		t.Fatal(err)
	}

	if err := c.routeAssets(&SSRAssets{
		ModuleName: "example.com/root",
		CSS:        ".root{color:red}",
		HTML:       `<main>Root</main>`,
		IsRoot:     true,
	}, true, false); err != nil {
		t.Fatal(err)
	}

	if err := c.RegenerateHTMLCache(); err != nil {
		t.Fatal(err)
	}
	html := string(c.GetCachedHTML())

	appOpen := `<div id="app">`
	appOpenIdx := strings.Index(html, appOpen)
	if appOpenIdx == -1 {
		t.Fatalf("html must contain <div id=\"app\">\nGot:\n%s", html)
	}
	closeIdx := strings.Index(html[appOpenIdx:], `</div>`)
	if closeIdx == -1 {
		t.Fatalf("html must contain </div> after <div id=\"app\">\nGot:\n%s", html)
	}
	between := html[appOpenIdx+len(appOpen) : appOpenIdx+closeIdx]

	for _, want := range []string{"Module", "Root"} {
		if !strings.Contains(between, want) {
			t.Errorf("markup %q must be inside #app\nbetween:\n%s", want, between)
		}
	}
	if strings.Contains(html[appOpenIdx+closeIdx:], "Root") {
		t.Errorf("root HTML must not appear after </div>\nGot:\n%s", html)
	}

	// El CSS del raíz conserva el slot close: gana la cascada sobre el CSS de
	// los módulos (que vive en middle).
	css, err := c.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	got := string(css)
	modIdx := strings.Index(got, ".mod{color:blue}")
	rootIdx := strings.Index(got, ".root{color:red}")
	if modIdx == -1 || rootIdx == -1 {
		t.Fatalf("expected both css rules in bundle, got %q", got)
	}
	if modIdx > rootIdx {
		t.Errorf("root CSS must come after module CSS to win the cascade, got %q", got)
	}
}

// Transición raíz → no-raíz para el mismo módulo: enforceSingleSlot limpia el
// slot "close" (CSS/JS) y el HTML re-registrado sigue cayendo en middle.
func TestRouteAssets_RootToNonRootTransition(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir()})

	if err := c.routeAssets(&SSRAssets{
		ModuleName: "example.com/root",
		CSS:        ".root{color:red}",
		HTML:       `<main>Root</main>`,
		IsRoot:     true,
	}, true, false); err != nil {
		t.Fatal(err)
	}

	if err := c.routeAssets(&SSRAssets{
		ModuleName: "example.com/root",
		CSS:        ".root{color:green}",
		HTML:       `<main>Root v2</main>`,
	}, false, false); err != nil {
		t.Fatal(err)
	}

	if err := c.RegenerateHTMLCache(); err != nil {
		t.Fatal(err)
	}
	html := string(c.GetCachedHTML())

	if strings.Count(html, "Root") != 1 {
		t.Errorf("HTML must appear exactly once after the transition, got %q", html)
	}
	if !strings.Contains(html, "Root v2") {
		t.Errorf("the updated root HTML must be present, got %q", html)
	}

	css, err := c.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(css), "color:red") {
		t.Errorf("stale CSS rule must be gone from the close slot, got %q", string(css))
	}
	if !strings.Contains(string(css), "color:green") {
		t.Errorf("updated CSS rule must be present, got %q", string(css))
	}
}