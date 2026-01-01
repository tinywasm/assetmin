package assetmin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Función auxiliar para crear archivos de módulos HTML de prueba
func createTestHtmlModules(t *testing.T, dir string) []string {
	moduleTemplate := `<div class="module-%d">
    <h2>Test Module %d</h2>
    <p>module %d content</p>
</div>`
	var paths []string
	for i := range 2 {
		moduleNumber := i + 1
		content := fmt.Sprintf(moduleTemplate, moduleNumber, moduleNumber, moduleNumber)
		path := filepath.Join(dir, fmt.Sprintf("module%d.html", moduleNumber))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test module: %v", err)
		}
		paths = append(paths, path)
	}
	return paths
}

// TestHtmlModulesIntegration verifica que la funcionalidad de integración de módulos HTML
// funciona correctamente, utilizando contentOpen para la apertura del HTML,
// contentMiddle para los módulos HTML y contentClose para el cierre.
func TestHtmlModulesIntegration(t *testing.T) {
	t.Run("uc09_html_modules_integration_without_index", func(t *testing.T) {
		// este test verifica que actualicen modulos html cunando no existe un documento html
		// principal. En este caso, el archivo index.html no existe y se espera que se genere uno por defecto

		env := setupTestEnv("uc09_html_modules_integration_without_index", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		// Crear un directorio de prueba para módulos HTML
		env.CreateModulesDir()

		// Crear archivos de módulos HTML individuales
		modulePaths := createTestHtmlModules(t, env.ModulesDir)

		// Procesar cada archivo de módulo
		for _, modulePath := range modulePaths {
			moduleName := filepath.Base(modulePath)
			if err := env.AssetsHandler.NewFileEvent(moduleName, ".html", modulePath, "create"); err != nil {
				t.Fatalf("Error processing HTML module creation event: %v", err)
			}
		}

		// Verificar que el archivo HTML principal fue creado
		if _, err := os.Stat(env.MainHtmlPath); os.IsNotExist(err) {
			t.Fatalf("El archivo index.html debería haberse creado at %s", env.MainHtmlPath)
		}

		// Leer el archivo generado
		content, err := os.ReadFile(env.MainHtmlPath)
		if err != nil {
			t.Fatalf("Debería poder leer el archivo HTML generado: %v", err)
		}

		// Verificar la estructura del contenido
		htmlContent := string(content)

		// Verificar que contenga la etiqueta de apertura HTML
		if !strings.Contains(htmlContent, "<!doctype html>") {
			t.Errorf("Debería contener la etiqueta doctype")
		}

		// Verificar que contenga los módulos HTML
		if !strings.Contains(htmlContent, "Test Module 1") {
			t.Errorf("Debería contener el módulo 1")
		}
		if !strings.Contains(htmlContent, "Test Module 2") {
			t.Errorf("Debería contener el módulo 2")
		}

		// Verificar que contenga la etiqueta de cierre
		if !strings.Contains(htmlContent, "</html>") {
			t.Errorf("Debería contener la etiqueta de cierre HTML")
		}

		// Probar eliminar un módulo 1
		if err := env.AssetsHandler.NewFileEvent("module1.html", ".html", modulePaths[0], "remove"); err != nil {
			t.Fatalf("Error processing HTML module removal: %v", err)
		}

		// Verificar que el HTML actualizado no contiene el módulo eliminado
		content, err = os.ReadFile(env.MainHtmlPath)
		if err != nil {
			t.Fatalf("Debería poder leer el archivo HTML actualizado: %v", err)
		}
		htmlContent = string(content)

		// El módulo eliminado no debería estar presente
		if strings.Contains(htmlContent, "Test Module 1") {
			t.Errorf("No debería contener el módulo 1 eliminado")
		}
		if !strings.Contains(htmlContent, "Test Module 2") {
			t.Errorf("Debería seguir conteniendo el módulo 2")
		}

		env.CleanDirectory()
	})
	t.Run("uc11_html_template_should_be_ignored", func(t *testing.T) {
		// Este test verifica que cuando existe un archivo template.html con estructura HTML completa
		// (que comienza con <!doctype html> y termina con </body></html>), este NO debe ser tratado
		// como un módulo HTML sino que debe ser ignorado para evitar duplicación de contenido

		env := setupTestEnv("uc11_html_template_should_be_ignored", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		// Crear un directorio de prueba para módulos HTML
		env.CreateModulesDir()

		// Crear un archivo template.html con estructura HTML completa
		// Este archivo NO debería incluirse como módulo en el index.html
		templateContent := `<!doctype html>
<html>
<head>
    <meta charset="utf-8">
    <title>Template Page</title>
    <link rel="stylesheet" href="style.css" type="text/css" />
</head>
<body>
    <header>
        <h1>Template Header</h1>
    </header>
    <main>
        <p>This is a complete template file</p>
    </main>
    <footer>
        <p>Template Footer</p>
    </footer>
    <script src="main.js" type="text/javascript"></script>
</body>
</html>`

		templatePath := filepath.Join(env.ModulesDir, "template.html")
		if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to write template HTML: %v", err)
		}

		// Crear un módulo HTML normal (fragmento sin estructura completa)
		moduleContent := `<div class="module-test">
    <h2>Test Module</h2>
    <p>This is a normal module fragment</p>
</div>`

		modulePath := filepath.Join(env.ModulesDir, "module1.html")
		if err := os.WriteFile(modulePath, []byte(moduleContent), 0644); err != nil {
			t.Fatalf("Failed to write module HTML: %v", err)
		}

		// Procesar el archivo template.html (que debería ser ignorado)
		if err := env.AssetsHandler.NewFileEvent("template.html", ".html", templatePath, "create"); err != nil {
			t.Fatalf("Error processing template event: %v", err)
		}

		// Procesar el módulo normal
		if err := env.AssetsHandler.NewFileEvent("module1.html", ".html", modulePath, "create"); err != nil {
			t.Fatalf("Error processing module creation: %v", err)
		}

		// Verificar que el archivo HTML principal fue creado
		if _, err := os.Stat(env.MainHtmlPath); os.IsNotExist(err) {
			t.Fatalf("El archivo index.html debería haberse creado at %s", env.MainHtmlPath)
		}

		// Leer el archivo generado
		content, err := os.ReadFile(env.MainHtmlPath)
		if err != nil {
			t.Fatalf("Debería poder leer el archivo HTML generado: %v", err)
		}

		htmlContent := string(content)

		// Verificar que contiene el módulo normal
		if !strings.Contains(htmlContent, "Test Module") {
			t.Errorf("Debería contener el módulo normal")
		}
		if !strings.Contains(htmlContent, "This is a normal module fragment") {
			t.Errorf("Debería contener el contenido del módulo normal")
		}

		// VERIFICAR QUE EL TEMPLATE.HTML NO SE INCLUYE COMO MÓDULO
		// No debe haber duplicación de estructura HTML
		if strings.Contains(htmlContent, "Template Header") {
			t.Errorf("NO debería contener el contenido del template.html")
		}
		if strings.Contains(htmlContent, "This is a complete template file") {
			t.Errorf("NO debería contener el contenido del template.html")
		}
		if strings.Contains(htmlContent, "Template Footer") {
			t.Errorf("NO debería contener el footer del template.html")
		}

		// Verificar que el HTML generado tiene la estructura correcta (solo una vez)
		doctypeCount := strings.Count(htmlContent, "<!doctype html>")
		if doctypeCount != 1 {
			t.Errorf("Solo debería haber un <!doctype html>, got %d", doctypeCount)
		}

		htmlOpenCount := strings.Count(htmlContent, "<html>")
		if htmlOpenCount != 1 {
			t.Errorf("Solo debería haber una etiqueta <html>, got %d", htmlOpenCount)
		}

		bodyCloseCount := strings.Count(htmlContent, "</body>")
		if bodyCloseCount != 1 {
			t.Errorf("Solo debería haber una etiqueta de cierre </body>, got %d", bodyCloseCount)
		}

		htmlCloseCount := strings.Count(htmlContent, "</html>")
		if htmlCloseCount != 1 {
			t.Errorf("Solo debería haber una etiqueta de cierre </html>, got %d", htmlCloseCount)
		}

		t.Logf("HTML Content:\n%s", htmlContent)

		env.CleanDirectory()
	})
}
