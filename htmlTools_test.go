package assetmin

import (
	"strings"
	"testing"
)

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

		open, close := parseExistingHtmlContent(html)

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

		open, close := parseExistingHtmlContent(html)

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

		open, close := parseExistingHtmlContent(html)

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

		open, close := parseExistingHtmlContent(html)

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

		open, close := parseExistingHtmlContent(html)

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
