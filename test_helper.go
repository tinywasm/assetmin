package assetmin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TestConcurrentFileProcessing is a reusable function to test concurrent file processing for both JS and CSS.
func (env *TestEnvironment) TestConcurrentFileProcessing(fileExtension string, fileCount int) {
	// Determine the file type and appropriate output path
	var outputPath string
	var fileType string

	switch fileExtension {
	case ".js":
		outputPath = env.MainJsPath
		fileType = "JS"
	case ".css":
		outputPath = env.MainCssPath
		fileType = "CSS"
	default:
		env.t.Fatalf("Unsupported file extension: %s", fileExtension)
	}

	// Create files with initial content
	fileNames := make([]string, fileCount)
	filePaths := make([]string, fileCount)
	fileContents := make([][]byte, fileCount)

	for i := range fileCount {
		fileNames[i] = fmt.Sprintf("file%d%s", i+1, fileExtension)
		filePaths[i] = filepath.Join(env.BaseDir, fileNames[i])

		// Generate appropriate content based on file type
		if fileExtension == ".js" {
			fileContents[i] = []byte(fmt.Sprintf("console.log('Content from %s file %d');", fileType, i+1))
		} else if fileExtension == ".css" {
			fileContents[i] = []byte(fmt.Sprintf(".test-class-%d { color: blue; content: \"Content from %s file %d\"; }", i+1, fileType, i+1))
		}
	}

	// Write initial files
	for i := range fileCount {
		if err := os.WriteFile(filePaths[i], fileContents[i], 0644); err != nil {
			env.t.Fatalf("Failed to write initial file %s: %v", filePaths[i], err)
		}
	}

	// Process files concurrently
	var wg sync.WaitGroup
	for i := range fileCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := env.AssetsHandler.NewFileEvent(fileNames[idx], fileExtension, filePaths[idx], "create"); err != nil {
				env.t.Errorf("Error processing file creation event for %s: %v", fileNames[idx], err)
			}
		}(i)
	}
	wg.Wait()

	// Verify the output file exists
	_, err := os.Stat(outputPath)
	if err != nil {
		env.t.Fatalf("The output file %s was not created for %s: %v", outputPath, fileType, err)
	}

	// Read the output file content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		env.t.Fatalf("Failed to read the output file for %s: %v", fileType, err)
	}

	// Verify that the content of all files is present
	contentStr := string(content)
	for i := range fileCount {
		expectedContent := fmt.Sprintf("Content from %s file %d", fileType, i+1)
		if !strings.Contains(contentStr, expectedContent) {
			env.t.Errorf("The content of %s file %d is not present in output", fileType, i+1)
		}
	}

	// Update all files with new content
	updatedContents := make([][]byte, fileCount)
	for i := range fileCount {
		// Generate updated content based on file type
		if fileExtension == ".js" {
			updatedContents[i] = []byte(fmt.Sprintf("console.log('Updated content from %s file %d');", fileType, i+1))
		} else if fileExtension == ".css" {
			updatedContents[i] = []byte(fmt.Sprintf(".test-class-%d { color: red; content: \"Updated content from %s file %d\"; }", i+1, fileType, i+1))
		}
		if err := os.WriteFile(filePaths[i], updatedContents[i], 0644); err != nil {
			env.t.Fatalf("Failed to update file %s: %v", filePaths[i], err)
		}
	}

	// Process the updated files concurrently
	wg = sync.WaitGroup{}
	for i := range fileCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := env.AssetsHandler.NewFileEvent(fileNames[idx], fileExtension, filePaths[idx], "write"); err != nil {
				env.t.Errorf("Error processing file write event for %s: %v", fileNames[idx], err)
			}
		}(i)
	}
	wg.Wait()

	// Read the updated output file content
	updatedContent, err := os.ReadFile(outputPath)
	if err != nil {
		env.t.Fatalf("Failed to read the updated output file for %s: %v", fileType, err)
	}
	updatedContentStr := string(updatedContent)

	// Verify that the updated content of all files is present
	for i := range fileCount {
		var expectedUpdatedContent string
		if fileExtension == ".js" {
			expectedUpdatedContent = fmt.Sprintf("Updated content from %s file %d", fileType, i+1)
		} else if fileExtension == ".css" {
			expectedUpdatedContent = fmt.Sprintf("content:\"Updated content from %s file %d\"", fileType, i+1)
		}
		if !strings.Contains(updatedContentStr, expectedUpdatedContent) {
			env.t.Errorf("The updated content of %s file %d is not present", fileType, i+1)
		}
	}

	// Verify that the original content is no longer present (no duplication)
	for i := range fileCount {
		var originalContent string
		if fileExtension == ".js" {
			originalContent = fmt.Sprintf("Content from %s file %d", fileType, i+1)
		} else if fileExtension == ".css" {
			originalContent = fmt.Sprintf("content:\"Content from %s file %d\"", fileType, i+1)
		}
		if strings.Contains(updatedContentStr, originalContent) {
			env.t.Errorf("The original content of %s file %d should not be present", fileType, i+1)
		}
	}
}

// TestFileCRUDOperations tests the complete CRUD cycle (create, write, remove) for a file
func (env *TestEnvironment) TestFileCRUDOperations(fileExtension string) {
	// Determine the file type and appropriate output path
	var outputPath string

	switch fileExtension {
	case ".js":
		outputPath = env.MainJsPath
	case ".css":
		outputPath = env.MainCssPath
	default:
		env.t.Fatalf("Unsupported file extension: %s", fileExtension)
	}

	// Create directories first
	env.CreatePublicDir()

	// 1. Create file with initial content
	fileName := fmt.Sprintf("script1%s", fileExtension)
	filePath := filepath.Join(env.BaseDir, fileName)
	var initialContent []byte

	if fileExtension == ".js" {
		initialContent = []byte("console.log('Initial content');")
	} else {
		initialContent = []byte(".test { color: blue; content: 'Initial content'; }")
	}

	if err := os.WriteFile(filePath, initialContent, 0644); err != nil {
		env.t.Fatalf("Failed to write initial file %s: %v", filePath, err)
	}
	if err := env.AssetsHandler.NewFileEvent(fileName, fileExtension, filePath, "create"); err != nil {
		env.t.Fatalf("Error processing file creation event: %v", err)
	}

	// Verify that the output file was created with the initial content
	_, err := os.Stat(outputPath)
	if err != nil {
		env.t.Fatalf("El archivo %s no fue creado: %v", outputPath, err)
	}
	initialOutputContent, err := os.ReadFile(outputPath)
	if err != nil {
		env.t.Fatalf("No se pudo leer el archivo %s: %v", outputPath, err)
	}
	if !strings.Contains(string(initialOutputContent), "Initial content") {
		env.t.Errorf("El contenido inicial no es el esperado en %s", outputPath)
	}

	// 2. Update the file content
	var updatedContent []byte
	if fileExtension == ".js" {
		updatedContent = []byte("console.log('Updated content');")
	} else {
		updatedContent = []byte(".test { color: red; content: 'Updated content'; }")
	}
	if err := os.WriteFile(filePath, updatedContent, 0644); err != nil {
		env.t.Fatalf("Failed to update file %s: %v", filePath, err)
	}
	if err := env.AssetsHandler.NewFileEvent(fileName, fileExtension, filePath, "write"); err != nil {
		env.t.Fatalf("Error processing file write event: %v", err)
	}

	// Verify the content was updated and not duplicated
	updatedOutputContent, err := os.ReadFile(outputPath)
	if err != nil {
		env.t.Fatalf("No se pudo leer el archivo %s actualizado: %v", outputPath, err)
	}
	if !strings.Contains(string(updatedOutputContent), "Updated content") {
		env.t.Errorf("El contenido actualizado no está presente en %s", outputPath)
	}
	if strings.Contains(string(updatedOutputContent), "Initial content") {
		env.t.Errorf("El contenido inicial no debería estar presente en %s", outputPath)
	}

	// 3. Remove the file
	if err := env.AssetsHandler.NewFileEvent(fileName, fileExtension, filePath, "remove"); err != nil {
		env.t.Fatalf("Error processing file removal event: %v", err)
	}

	// Verify the content was removed
	finalOutputContent, err := os.ReadFile(outputPath)
	if err != nil {
		env.t.Fatalf("No se pudo leer el archivo %s después de eliminar: %v", outputPath, err)
	}
	if strings.Contains(string(finalOutputContent), "Updated content") {
		env.t.Errorf("El contenido eliminado no debería estar presente en %s", outputPath)
	}
}
