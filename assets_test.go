package assetmin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssetScenario(t *testing.T) {

	t.Run("uc01_empty_directory", func(t *testing.T) {
		// en este caso se espera que la libreria pueda crear el archivo el el directorio web/public/main.js
		// si el archivo no existe se considerara un error, la libreria debe ser capas de crear el directorio de trabajo web/public

		env := setupTestEnv("uc01_empty_directory", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		// 1. Create JS file and verify output
		jsFileName := "script1.js"
		jsFilePath := filepath.Join(env.BaseDir, jsFileName)
		jsContent := []byte("console.log('Hello from JS');")

		require.NoError(t, os.WriteFile(jsFilePath, jsContent, 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "create"))

		// Verificar que el archivo main.js fue creado correctamente
		_, err := os.Stat(env.MainJsPath)
		require.NoError(t, err, "El archivo main.js no fue creado")
		require.FileExists(t, env.MainJsPath, "El archivo main.js no existe")

		// Verificar que el contenido fue escrito correctamente
		content, err := os.ReadFile(env.MainJsPath)
		require.NoError(t, err, "No se pudo leer el archivo main.js")
		require.Contains(t, string(content), "Hello from JS", "El contenido del archivo main.js no es el esperado")

		env.CleanDirectory()

	})

	t.Run("uc02_crud_operations", func(t *testing.T) {
		// En este caso probamos operaciones CRUD (Create, Read, Update, Delete) en archivos
		// Se espera que el contenido se actualice correctamente (sin duplicados) y
		// que el contenido sea eliminado cuando se elimina el archivo
		env := setupTestEnv("uc02_crud_operations", t)
		env.AssetsHandler.SetBuildOnDisk(true)
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

	t.Run("uc03_concurrent_writes", func(t *testing.T) {
		// En este caso probamos el comportamiento de la librería cuando múltiples
		// archivos JS son escritos simultáneamente
		// Se espera que todos los contenidos se encuentren en web/public/main.js
		env := setupTestEnv("uc03_concurrent_writes", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.TestConcurrentFileProcessing(".js", 5)
		env.CleanDirectory()
	})

	t.Run("uc04_concurrent_writes_css", func(t *testing.T) {
		// En este caso probamos el comportamiento de la librería cuando múltiples
		// archivos CSS son escritos simultáneamente
		// Se espera que todos los contenidos se encuentren en web/public/main.css
		env := setupTestEnv("uc04_concurrent_writes_css", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.TestConcurrentFileProcessing(".css", 5)
		env.CleanDirectory()
	})
}

func (env *TestEnvironment) TestEventBasedCompilation(fileExtension string) {
	var mainPath, fileName, fileContent, expectedContent string

	if fileExtension == ".js" {
		mainPath = env.MainJsPath
		fileName = "script1.js"
		fileContent = "console.log('JS content');"
		expectedContent = "JS content"
	} else {
		mainPath = env.MainCssPath
		fileName = "style1.css"
		fileContent = "body { color: blue; }"
		expectedContent = "body{color:blue}"
	}

	filePath := filepath.Join(env.BaseDir, fileName)
	require.NoError(env.t, os.WriteFile(filePath, []byte(fileContent), 0644))

	// --- false Behavior ---
	env.AssetsHandler.SetBuildOnDisk(false)
	require.NoError(env.t, env.AssetsHandler.NewFileEvent(fileName, fileExtension, filePath, "create"))

	// Verify file is NOT written to disk in false
	_, err := os.Stat(mainPath)
	require.True(env.t, os.IsNotExist(err), "File should not be written in false")

	// --- true Behavior ---
	env.AssetsHandler.SetBuildOnDisk(true)
	require.NoError(env.t, env.AssetsHandler.NewFileEvent(fileName, fileExtension, filePath, "write"))

	// Verify file IS written to disk in true
	require.FileExists(env.t, mainPath, "File should be written in true")
	content, err := os.ReadFile(mainPath)
	require.NoError(env.t, err)
	require.Contains(env.t, string(content), expectedContent, "File content mismatch in true")
}
