package assetmin

import "strings"

type htmlHandler struct {
	*asset
	cssURL string
	jsURL  string
}

// generateStylesheetLink returns HTML tag for linking a CSS stylesheet
func (h *htmlHandler) generateStylesheetLink() []byte {
	return []byte(`<link rel="stylesheet" href="` + h.cssURL + `" type="text/css" />`)
}

// generateJavaScriptTag returns HTML script tag for a JavaScript file
func (h *htmlHandler) generateJavaScriptTag() []byte {
	return []byte(`<script src="` + h.jsURL + `" type="text/javascript"></script>`)
}

// NewHtmlHandler creates an HTML asset handler using the provided output filename
func NewHtmlHandler(ac *Config, outputName, cssURL, jsURL string) *asset {
	af := newAssetFile(outputName, "text/html", ac, nil)

	hh := &htmlHandler{
		asset:  af,
		cssURL: cssURL,
		jsURL:  jsURL,
	}
	//  default marcador de inicio index HTML
	af.contentOpen = append(af.contentOpen, &contentFile{
		path: "index-open.html",
		content: []byte(`<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<title></title>
	` + string(hh.generateStylesheetLink()) + `
</head>
<body>`),
	})

	// default marcador de cierre index HTML
	af.contentClose = append(af.contentClose, &contentFile{
		path: "index-close.html",
		content: []byte(string(hh.generateJavaScriptTag()) + `
</body>
</html>`),
	})

	return af
}

// parseExistingHtmlContent analiza un archivo HTML existente para identificar
// las secciones de apertura y cierre, considerando dónde deben insertarse los módulos
func parseExistingHtmlContent(content string) (openContent, closeContent string) {
	// Buscar un marcador explícito de comentario
	if i := strings.Index(content, "<!-- MODULES_PLACEHOLDER -->"); i != -1 {
		return content[:i], content[i+len("<!-- MODULES_PLACEHOLDER -->"):]
	}

	// Buscar un marcador de plantilla Go
	if i := strings.Index(content, "{{.Modules}}"); i != -1 {
		return content[:i], content[i+len("{{.Modules}}"):]
	}

	lines := strings.Split(content, "\n")
	var splitIndex int

	// 1. Buscar dentro de un tag <main> si existe
	inMain := false
	for i, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))

		if strings.Contains(lineLower, "<main") {
			inMain = true
			continue
		}

		if inMain && strings.Contains(lineLower, "</main>") {
			// Colocar el índice antes del cierre de main para que los módulos
			// se inserten dentro del tag main
			splitIndex = i
			break
		}
	}

	// 2. Si no se encontró dentro de <main>, buscar antes del primer <script>
	if splitIndex == 0 {
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), "<script") {
				splitIndex = i
				break
			}
		}
	}

	// 3. Si no hay <script>, buscar antes de </body>
	if splitIndex == 0 {
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), "</body>") {
				splitIndex = i
				break
			}
		}
	}

	// Si todavía no tenemos un punto, usar el final
	if splitIndex == 0 {
		splitIndex = len(lines)
	}

	return strings.Join(lines[:splitIndex], "\n"), strings.Join(lines[splitIndex:], "\n")
}
