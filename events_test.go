package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOutputFileHandling verifica que la función NewFileEvent maneje correctamente los casos
// donde el archivo de entrada es uno de los archivos de salida (main.js o main.css).
// Esto evita la recursión infinita o bucles de procesamiento cuando el sistema de archivos
// notifica cambios en los archivos generados por la propia librería.
// Se prueban diferentes formatos de rutas para sistemas Windows, Mac y Linux,
// así como comparaciones de rutas sin distinción entre mayúsculas y minúsculas.
func TestOutputFileHandling(t *testing.T) {
	t.Run("uc00_output_file_handling", func(t *testing.T) {
		env := setupTestEnv("uc00_output_file_handling", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir() // Ensure public directory exists

		// Desactivar el código de inicialización durante esta prueba para evitar conflictos
		originalInitFunc := env.AssetsHandler.GetSSRClientInitJS
		env.AssetsHandler.GetSSRClientInitJS = func() (string, error) {
			return "console.log('init');", nil
		}
		defer func() {
			// Restaurar la función original al finalizar
			env.AssetsHandler.GetSSRClientInitJS = originalInitFunc
		}()

		// Create an initial JS file to ensure main.js is created
		initialJsFile := filepath.Join(env.BaseDir, "initial.js")
		if err := os.WriteFile(initialJsFile, []byte("console.log('initial');"), 0644); err != nil {
			t.Fatalf("Failed to write initial JS file: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("initial.js", ".js", initialJsFile, "create"); err != nil {
			t.Fatalf("Error processing creation event: %v", err)
		}

		// Verify main.js exists
		if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
			t.Fatalf("The script.js file should be created at %s", env.MainJsPath)
		}

		// Test that directly modifying the output file doesn't cause issues
		// Test for Windows-style path
		t.Run("windows_path", func(t *testing.T) {
			windowsPath := filepath.FromSlash(env.MainJsPath)
			err := env.AssetsHandler.NewFileEvent("script.js", ".js", windowsPath, "write")
			if err != nil {
				t.Errorf("Should handle Windows output path without error: %v", err)
			}
		})

		// Test for Unix-style path
		t.Run("unix_path", func(t *testing.T) {
			unixPath := filepath.ToSlash(env.MainJsPath)
			err := env.AssetsHandler.NewFileEvent("script.js", ".js", unixPath, "write")
			if err != nil {
				t.Errorf("Should handle Unix output path without error: %v", err)
			}
		})

		// Test for CSS output file
		t.Run("css_output_file", func(t *testing.T) {
			// Create an initial CSS file to ensure main.css is created
			initialCssFile := filepath.Join(env.BaseDir, "initial.css")
			if err := os.WriteFile(initialCssFile, []byte("body { color: red; }"), 0644); err != nil {
				t.Fatalf("Failed to write initial CSS file: %v", err)
			}
			if err := env.AssetsHandler.NewFileEvent("initial.css", ".css", initialCssFile, "create"); err != nil {
				t.Fatalf("Error processing CSS creation event: %v", err)
			}

			// Verify main.css exists
			if _, err := os.Stat(env.MainCssPath); os.IsNotExist(err) {
				t.Fatalf("The main.css file should be created at %s", env.MainCssPath)
			}

			// Test the CSS output file
			err := env.AssetsHandler.NewFileEvent("main.css", ".css", env.MainCssPath, "write")
			if err != nil {
				t.Errorf("Should handle CSS output path without error: %v", err)
			}
		})

		// Test for case-insensitive path comparison
		t.Run("case_insensitive_path", func(t *testing.T) {
			// On Windows, file paths are case-insensitive, test that here
			lowerCasePath := filepath.ToSlash(env.MainJsPath)
			upperCasePath := lowerCasePath
			// Convert just the filename part to uppercase
			dir, _ := filepath.Split(upperCasePath)
			upperCasePath = filepath.Join(dir, "SCRIPT.JS")

			err := env.AssetsHandler.NewFileEvent("SCRIPT.JS", ".js", upperCasePath, "write")
			// This should still be handled properly
			if err != nil {
				t.Errorf("Should handle case differences in path without error: %v", err)
			}
		})

		env.CleanDirectory()
	})
}
