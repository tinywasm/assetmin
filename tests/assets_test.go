//go:build !wasm

package assetmin_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssets(t *testing.T) {
	// Configurar el entorno de prueba
	t.Run("uc01_basic_flow", func(t *testing.T) {
		// En este caso probamos que el flujo básico de creación de activos funcione
		// Se crean archivos .js, .css, .svg y .html y se verifica que los handlers
		// tengan el contenido correcto y que se puedan minificar sin errores.
		env := setupTestEnv("uc01_basic_flow", t)
		env.CreateFile("style.css", "body { color: red; }")
		env.CreateFile("script.js", "console.log('hello');")
		env.CreateFile("icon.svg", "<svg><path d='M0 0h10v10H0z'/></svg>")
		env.CreateFile("index.html", "<html><body><h1>Hello</h1></body></html>")

		if err := env.AssetsHandler.FlushToDisk(); err != nil {
			t.Fatalf("FlushToDisk: %v", err)
		}

		// Verificar que los archivos existan en el directorio de salida
		for _, file := range []string{"style.css", "script.js", "icons.svg", "index.html"} {
			path := filepath.Join(env.OutDir, file)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("File %s not found in output directory", file)
			}
		}
		env.CleanDirectory()
	})

	t.Run("uc02_crud_operations", func(t *testing.T) {
		// En este caso probamos operaciones CRUD (Create, Read, Update, Delete) en archivos
		// Se espera que el contenido se actualice correctamente (sin duplicados) y
		// que el contenido sea eliminado cuando se elimina el archivo
		env := setupTestEnv("uc02_crud_operations", t)
		if err := env.AssetsHandler.FlushToDisk(); err != nil {
			t.Fatalf("FlushToDisk: %v", err)
		}

		// Probar operaciones CRUD para archivos JS
		t.Run("js_file", func(t *testing.T) {
			env.TestFileCRUDOperations(".js")
		})

		// Probar operaciones CRUD para archivos CSS
		t.Run("css_file", func(t *testing.T) {
			env.TestFileCRUDOperations(".css")
		})

		env.CleanDirectory()
	})
}
