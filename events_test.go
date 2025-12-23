package assetmin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		originalInitFunc := env.AssetsHandler.GetRuntimeInitializerJS
		env.AssetsHandler.GetRuntimeInitializerJS = func() (string, error) {
			return "", nil
		}
		defer func() {
			// Restaurar la función original al finalizar
			env.AssetsHandler.GetRuntimeInitializerJS = originalInitFunc
		}()

		// Create an initial JS file to ensure main.js is created
		initialJsFile := filepath.Join(env.BaseDir, "initial.js")
		require.NoError(t, os.WriteFile(initialJsFile, []byte("console.log('initial');"), 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent("initial.js", ".js", initialJsFile, "create"))

		// Verify main.js exists
		require.FileExists(t, env.MainJsPath, "The script.js file should be created")

		// Test that directly modifying the output file doesn't cause issues
		// Test for Windows-style path
		t.Run("windows_path", func(t *testing.T) {
			windowsPath := filepath.FromSlash(env.MainJsPath)
			err := env.AssetsHandler.NewFileEvent("script.js", ".js", windowsPath, "write")
			assert.NoError(t, err, "Should handle Windows output path without error")
		})

		// Test for Unix-style path
		t.Run("unix_path", func(t *testing.T) {
			unixPath := filepath.ToSlash(env.MainJsPath)
			err := env.AssetsHandler.NewFileEvent("script.js", ".js", unixPath, "write")
			assert.NoError(t, err, "Should handle Unix output path without error")
		})

		// Test for CSS output file
		t.Run("css_output_file", func(t *testing.T) {
			// Create an initial CSS file to ensure main.css is created
			initialCssFile := filepath.Join(env.BaseDir, "initial.css")
			require.NoError(t, os.WriteFile(initialCssFile, []byte("body { color: red; }"), 0644))
			require.NoError(t, env.AssetsHandler.NewFileEvent("initial.css", ".css", initialCssFile, "create"))

			// Verify main.css exists
			require.FileExists(t, env.MainCssPath, "The main.css file should be created")

			// Test the CSS output file
			err := env.AssetsHandler.NewFileEvent("main.css", ".css", env.MainCssPath, "write")
			assert.NoError(t, err, "Should handle CSS output path without error")
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
			assert.NoError(t, err, "Should handle case differences in path without error")
		})

		env.CleanDirectory()
	})
}
