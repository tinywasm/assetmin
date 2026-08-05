//go:build !wasm

package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/assetmin"
)

// Canonical viewport content. Both shells (html.go NewHtmlHandler and
// templates/index_basic.html) must emit this exact string — see
// docs/ARCHITECTURE.md "document shell is duplicated".
const viewportContent = "width=device-width, initial-scale=1, viewport-fit=cover"

func TestParseExistingHtmlContent(t *testing.T) {
	t.Run("with_placeholder", func(t *testing.T) {
		html := `<!doctype html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <header>Header</header>
    <!-- MODULES_PLACEHOLDER -->
    <footer>Footer</footer>
    <script src="app.js"></script>
</body>
</html>`

		open, close := assetmin.ParseExistingHtmlContent(html)

		if !strings.Contains(open, "<header>Header</header>") {
			t.Errorf("open should contain header")
		}
		if !strings.Contains(close, "<footer>Footer</footer>") {
			t.Errorf("close should contain footer")
		}
		if !strings.Contains(close, "<script src=\"app.js\"></script>") {
			t.Errorf("close should contain script tag")
		}
	})

	t.Run("with_main_tag", func(t *testing.T) {
		html := `<!doctype html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <header>Header</header>
    <main>
        <div>Content</div>
    </main>
    <footer>Footer</footer>
    <script src="app.js"></script>
</body>
</html>`

		open, close := assetmin.ParseExistingHtmlContent(html)

		if !strings.Contains(open, "<main>") {
			t.Errorf("open should contain <main>")
		}
		if !strings.Contains(close, "</main>") {
			t.Errorf("close should contain </main>")
		}
		if !strings.Contains(close, "<footer>Footer</footer>") {
			t.Errorf("close should contain footer")
		}
	})

	t.Run("with_script_tag", func(t *testing.T) {
		html := `<!doctype html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <header>Header</header>
    <div>Content</div>
    <script src="app.js"></script>
</body>
</html>`

		open, close := assetmin.ParseExistingHtmlContent(html)

		if !strings.Contains(open, "<div>Content</div>") {
			t.Errorf("open should contain content div")
		}
		if !strings.Contains(close, "<script src=\"app.js\"></script>") {
			t.Errorf("close should contain script tag")
		}
		if strings.Contains(open, "<script") {
			t.Errorf("open should NOT contain script tag")
		}
	})

	t.Run("only_body_tag", func(t *testing.T) {
		html := `<!doctype html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <header>Header</header>
    <div>Content</div>
</body>
</html>`

		open, close := assetmin.ParseExistingHtmlContent(html)

		if !strings.Contains(open, "<div>Content</div>") {
			t.Errorf("open should contain content div")
		}
		if !strings.Contains(close, "</body>") {
			t.Errorf("close should contain </body>")
		}
		if !strings.Contains(close, "</html>") {
			t.Errorf("close should contain </html>")
		}
	})

	t.Run("complex_body_structure", func(t *testing.T) {
		html := `<!DOCTYPE html>
<html lang="es">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="StyleSheet" href="style.css">
	<title>App Title</title>
</head>
<body>
	<nav class="menu-container">
		<ul class="navbar-container">
			<li class="navbar-item">
				<a href="#" class="navbar-link">Home</a>
			</li>
		</ul>
	</nav>
	<header>
		<div id="USER_NAME"><a href="#login">Username</a></div>
		<h2 id="USER_AREA">User Area</h2>
	</header>
	<div id="user-mobile-messages">
		<h4 class="err">Message</h4>
	</div>

	{{.Modules}}

	<script src="app.js"></script>
</body>
</html>`

		open, close := assetmin.ParseExistingHtmlContent(html)

		// Verificar que el contenido se dividió correctamente en el marcador {{.Modules}}
		if !strings.Contains(open, "<div id=\"user-mobile-messages\">") {
			t.Errorf("open should contain user-mobile-messages div")
		}
		if !strings.Contains(open, "<h4 class=\"err\">Message</h4>") {
			t.Errorf("open should contain error message")
		}
		if !strings.Contains(open, `<div id="user-mobile-messages">
		<h4 class="err">Message</h4>
	</div>`) {
			t.Errorf("open should contain full message structure")
		}
		if !strings.Contains(close, "<script src=\"app.js\"></script>") {
			t.Errorf("close should contain script tag")
		}

		// Verificar que la división fue exacta alrededor del marcador
		if !strings.HasSuffix(strings.TrimSpace(open), "</div>") {
			t.Errorf("open should end with </div>")
		}
		if !strings.HasPrefix(strings.TrimSpace(close), "<script") {
			t.Errorf("close should start with <script")
		}
	})
}

// TestDefaultHTMLContainsAppDiv verifies that the default HTML template injected by
// NewHtmlHandler includes <div id="app"> as the WASM mount point.
// This ensures the SVG sprite (injected inline before the div) survives when
// the WASM client calls Render("app", ...) instead of Render("body", ...).
func TestDefaultHTMLContainsAppDiv(t *testing.T) {
	env := setupTestEnv("html_app_div", t)
	am := env.AssetsHandler

	if err := am.RegenerateHTMLCache(); err != nil {
		t.Fatalf("RegenerateHTMLCache: %v", err)
	}

	html := string(am.GetCachedHTML())

	if !strings.Contains(html, `<div id="app">`) && !strings.Contains(html, `<div id="app"/>`) {
		t.Errorf("HTML template must contain <div id=\"app\"> as WASM mount point.\nGot:\n%s", html)
	}

	// The sprite must come before the app div so it is not replaced by Render("app", ...)
	spriteIdx := strings.Index(html, `class="sprite-icons"`)
	appIdx := strings.Index(html, `id="app"`)

	if spriteIdx != -1 && appIdx != -1 && spriteIdx > appIdx {
		t.Errorf("SVG sprite must appear before <div id=\"app\"> in the HTML")
	}
}

// Without viewport-fit=cover, env(safe-area-inset-*) is 0px on every device
// (including notched iPhones). The CSS tokens that consume those values then
// compile and serve but do nothing — a silent layout failure.
func TestDefaultHTMLContainsViewportFitCover(t *testing.T) {
	env := setupTestEnv("html_viewport_fit", t)
	am := env.AssetsHandler

	if err := am.RegenerateHTMLCache(); err != nil {
		t.Fatalf("RegenerateHTMLCache: %v", err)
	}

	html := string(am.GetCachedHTML())
	// Minify may drop spaces inside the attribute; the token itself must survive.
	if !strings.Contains(html, "viewport-fit=cover") {
		t.Errorf("generated index.html must contain viewport-fit=cover\nGot:\n%s", html)
	}
}

// While assetmin keeps two HTML shells (NewHtmlHandler + templates/index_basic.html),
// their viewport content attribute must be the same string. Divergence is two
// truths about the same document.
func TestViewportContentIdenticalInBothShells(t *testing.T) {
	root := moduleRoot(t)

	handlerSrc, err := os.ReadFile(filepath.Join(root, "html.go"))
	if err != nil {
		t.Fatalf("read html.go: %v", err)
	}
	tmplSrc, err := os.ReadFile(filepath.Join(root, "templates", "index_basic.html"))
	if err != nil {
		t.Fatalf("read templates/index_basic.html: %v", err)
	}

	handlerVP := extractViewportContent(string(handlerSrc))
	tmplVP := extractViewportContent(string(tmplSrc))
	if handlerVP == "" {
		t.Fatal("html.go: no viewport content attribute found")
	}
	if tmplVP == "" {
		t.Fatal("templates/index_basic.html: no viewport content attribute found")
	}
	if handlerVP != tmplVP {
		t.Errorf("viewport content diverged:\n  html.go:                     %q\n  templates/index_basic.html:  %q", handlerVP, tmplVP)
	}
	if handlerVP != viewportContent {
		t.Errorf("viewport content = %q, want %q", handlerVP, viewportContent)
	}
}

func extractViewportContent(src string) string {
	const marker = `name="viewport" content="`
	i := strings.Index(src, marker)
	if i == -1 {
		return ""
	}
	start := i + len(marker)
	end := strings.Index(src[start:], `"`)
	if end == -1 {
		return ""
	}
	return src[start : start+end]
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}
