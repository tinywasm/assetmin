//go:build !wasm

package assetmin_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tinywasm/router/mock"
)

// spriteMarkup identifica el sprite SVG inyectado por AddDynamicContent.
const spriteMarkup = `aria-hidden="true" style="display:none"`

// TestModuleHTMLInsideApp es la traba del plan: el RenderHTML() de un módulo
// debe caer DENTRO de `<div id="app">` — para que la primera pintura del
// servidor ya muestre el markup que el WASM va a renderizar — y los símbolos
// SVG deben quedar FUERA de #app, o el primer Render() los borraría.
func TestModuleHTMLInsideApp(t *testing.T) {
	env := setupTestEnv("html_inside_app", t)
	am := env.AssetsHandler

	if err := am.InjectSpriteIcon("test-icon", "<path d='M0 0h1'/>", "0 0 16 16"); err != nil {
		t.Fatal(err)
	}

	modHTML := `<header>Module</header><main>Content</main>`
	if err := am.UpdateSSRModule("example.com/mod", "", nil, modHTML, nil); err != nil {
		t.Fatal(err)
	}

	if err := am.RegenerateHTMLCache(); err != nil {
		t.Fatal(err)
	}
	got := string(am.GetCachedHTML())

	appOpen := `<div id="app">`
	appOpenIdx := strings.Index(got, appOpen)
	if appOpenIdx == -1 {
		t.Fatalf("html must contain <div id=\"app\"> as mount point\nGot:\n%s", got)
	}
	closeIdx := strings.Index(got[appOpenIdx:], `</div>`)
	if closeIdx == -1 {
		t.Fatalf("html must contain </div> after <div id=\"app\">\nGot:\n%s", got)
	}
	between := got[appOpenIdx+len(appOpen) : appOpenIdx+closeIdx]

	if !strings.Contains(between, modHTML) {
		t.Errorf("module markup must be BETWEEN <div id=\"app\"> and </div>\nbetween:\n%s\nfull:\n%s", between, got)
	}

	// Los símbolos SVG quedan antes de <div id="app">
	spriteIdx := strings.Index(got, spriteMarkup)
	if spriteIdx == -1 {
		t.Fatalf("html must contain the SVG sprite\nGot:\n%s", got)
	}
	if spriteIdx > appOpenIdx {
		t.Error("SVG sprite must appear BEFORE <div id=\"app\"> so the first Render() cannot erase it")
	}

	// El cierre </div> debe preceder al <script> del bundle
	scriptIdx := strings.Index(got, `<script`)
	if scriptIdx != -1 && appOpenIdx+closeIdx > scriptIdx {
		t.Error("</div> must close #app before the script tag")
	}
}

// TestIndexHTMLIsSessionIndependent es la guarda del invariante del plan:
// el index.html se genera una sola vez en build y se sirve idéntico a todo el
// mundo. No hay render por request — dos instancias con el mismo registro
// deben producir bytes idénticos, y lo que FlushToDisk escribe a disco debe
// ser exactamente lo que se sirve en memoria. Si algo dependiera de estado de
// sesión, este test no podría pasar.
func TestIndexHTMLIsSessionIndependent(t *testing.T) {
	build := func() string {
		env := setupTestEnv("session_invariant", t)
		am := env.AssetsHandler
		if err := am.UpdateSSRModule("example.com/mod", ".mod{color:red}", nil, `<main>Static</main>`, nil); err != nil {
			t.Fatal(err)
		}
		if err := am.RegenerateHTMLCache(); err != nil {
			t.Fatal(err)
		}
		return string(am.GetCachedHTML())
	}

	first := build()
	second := build()
	if first != second {
		t.Error("index.html must be deterministic: two identical registrations produced different bytes")
	}

	// Lo que se sirve por HTTP == lo que queda en memoria == lo que FlushToDisk
	// escribe a disco: un solo artefacto de build, cacheable por definición.
	env := setupTestEnv("session_invariant_disk", t)
	am := env.AssetsHandler
	if err := am.UpdateSSRModule("example.com/mod", ".mod{color:red}", nil, `<main>Static</main>`, nil); err != nil {
		t.Fatal(err)
	}
	if err := am.FlushToDisk(); err != nil {
		t.Fatal(err)
	}

	disk, err := os.ReadFile(am.GetMainHtmlPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != first {
		t.Error("FlushToDisk must write exactly the same index.html bytes that are served in memory")
	}

	// Dos peticiones HTTP distintas reciben el mismo documento.
	r := newTestRouter(am)
	for _, p := range []string{"/", "/"} {
		ctx := &mock.Context{
			InPath:   p,
			InMethod: "GET",
		}
		r.Invoke("GET", p, ctx)
		if got := string(ctx.ResponseBody()); got != first {
			t.Error("served index.html must be byte-identical to the built artifact")
		}
	}
}