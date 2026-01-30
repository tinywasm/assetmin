package assetmin

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (c *AssetMin) UpdateFileContentInMemory(filePath, extension, event string, content []byte) (*asset, error) {
	file := &contentFile{
		path:    filePath,
		content: content,
	}

	switch extension {
	case ".css":
		err := c.mainStyleCssHandler.UpdateContent(filePath, event, file)
		return c.mainStyleCssHandler, err

	case ".js":
		// Remove a leading "use strict" directive from incoming files to avoid
		// duplicating the directive which we add globally in startCodeJS.
		file.content = stripLeadingUseStrict(file.content)
		err := c.mainJsHandler.UpdateContent(filePath, event, file)
		return c.mainJsHandler, err

	case ".svg":
		// Check if it's the favicon file
		if filepath.Base(filePath) == c.faviconSvgHandler.fileOutputName {
			err := c.faviconSvgHandler.UpdateContent(filePath, event, file)
			return c.faviconSvgHandler, err
		}
		// Otherwise treat as sprite icon
		err := c.spriteSvgHandler.UpdateContent(filePath, event, file)
		return c.spriteSvgHandler, err

	case ".html":
		err := c.indexHtmlHandler.UpdateContent(filePath, event, file)
		return c.indexHtmlHandler, err
	}

	return nil, errors.New("UpdateFileContentInMemory extension: " + extension + " not found " + filePath)
}

// event: create, remove, write, rename
func (c *AssetMin) NewFileEvent(fileName, extension, filePath, event string) error {
	// In SSR mode, delegate to external server and return early
	if c.isSSRMode() {
		return c.onSSRCompile()
	}

	// Check if filePath matches any of our output paths to avoid infinite recursion
	if c.isOutputPath(filePath) {
		//c.writeMessage("Skipping output file:", filePath)
		return nil
	}

	c.mu.Lock()         // Lock the mutex at the beginning
	defer c.mu.Unlock() // Ensure mutex is unlocked when the function returns

	var e = "NewFileEvent " + extension + " " + event
	if filePath == "" {
		return errors.New(e + "filePath is empty")
	}

	c.writeMessage(event, filePath)

	// Increase sleep duration significantly to allow file system operations (like write after rename) to settle
	// fail when time is < 10ms
	time.Sleep(20 * time.Millisecond) // Increased from 10ms

	var content []byte
	var err error

	// For delete/remove events, we don't need to read file content since file no longer exists
	if event == "remove" || event == "delete" {
		content = []byte{} // Empty content for delete events
	} else {
		// read file content from filePath for other events
		content, err = os.ReadFile(filePath)
		if err != nil {
			return errors.New(e + err.Error())
		}
	}

	fh, err := c.UpdateFileContentInMemory(filePath, extension, event, content) // Update contentMiddle
	if err != nil {
		return errors.New(e + err.Error())
	}
	if fh == nil {
		return nil
	}

	return c.processAsset(fh)
}

func (c *AssetMin) processAsset(fh *asset) error {
	// 1. Always regenerate cache
	if err := fh.RegenerateCache(c.min); err != nil {
		return err
	}

	// 2. Write to disk only if enabled
	if c.buildOnDisk {
		return FileWrite(fh.outputPath, *bytes.NewBuffer(fh.GetCachedMinified()))
	}
	return nil
}

func (c *AssetMin) UnobservedFiles() []string {
	// Only truly generated/merged files should be unobserved.
	// index.html and favicon.svg are often user-editable.
	return []string{
		c.mainStyleCssHandler.outputPath,
		c.mainJsHandler.outputPath,
		c.spriteSvgHandler.outputPath,
	}
}

func (c *AssetMin) startCodeJS() (out string, err error) {
	out = "'use strict';"

	if c.GetSSRClientInitJS == nil {
		return out, nil
	}

	js, err := c.GetSSRClientInitJS() // wasm js code
	if err != nil {
		return "", errors.New("startCodeJS " + err.Error())
	}

	// Remove any leading 'use strict' in the initializer to avoid duplication.
	// The initializer comes from GetRuntimeInitializerJS and doesn't go through
	// UpdateFileContentInMemory, so we need to clean it here.
	clean := stripLeadingUseStrict([]byte(js))
	out += string(clean)

	return
}

// clear memory files
func (f *asset) ClearMemoryFiles() {
	f.contentOpen = []*contentFile{}
	f.contentMiddle = []*contentFile{}
	f.contentClose = []*contentFile{}
}

// ShouldCompileToWasm checks if the file triggers WASM compilation.
// AssetMin handles assets, not WASM, so always returns false.
func (c *AssetMin) ShouldCompileToWasm(fileName, filePath string) bool {
	return false
}

// MainInputFileRelativePath returns the main input file path.
// AssetMin manages multiple assets, so returns empty.
// Used by DevWatch for specific file watching logic.
func (c *AssetMin) MainInputFileRelativePath() string {
	return ""
}

// MainOutputFileAbsolutePath returns the main output file path.
// AssetMin manages multiple outputs, so returns empty.
// Used by DevWatch for exclusion logic (handled by UnobservedFiles instead)
func (c *AssetMin) MainOutputFileAbsolutePath() string {
	return ""
}

// isOutputPath checks if the given file path matches any of our output paths
func (c *AssetMin) isOutputPath(filePath string) bool {
	// Normalize paths for cross-platform comparison
	normalizedFilePath := filepath.Clean(filePath)
	cssOutputPath := filepath.Clean(c.mainStyleCssHandler.outputPath)
	jsOutputPath := filepath.Clean(c.mainJsHandler.outputPath)
	svgOutputPath := filepath.Clean(c.spriteSvgHandler.outputPath)
	faviconOutputPath := filepath.Clean(c.faviconSvgHandler.outputPath)
	htmlHandlerOutputPath := filepath.Clean(c.indexHtmlHandler.outputPath)

	// Case-sensitive comparison first
	if normalizedFilePath == cssOutputPath ||
		normalizedFilePath == jsOutputPath ||
		normalizedFilePath == svgOutputPath ||
		normalizedFilePath == faviconOutputPath ||
		normalizedFilePath == htmlHandlerOutputPath {
		return true
	}

	// Case-insensitive comparison for cross-platform compatibility
	normalizedFilePathLower := strings.ToLower(normalizedFilePath)
	cssOutputPathLower := strings.ToLower(cssOutputPath)
	jsOutputPathLower := strings.ToLower(jsOutputPath)
	svgOutputPathLower := strings.ToLower(svgOutputPath)
	faviconOutputPathLower := strings.ToLower(faviconOutputPath)
	htmlHandlerOutputPathLower := strings.ToLower(htmlHandlerOutputPath)

	return normalizedFilePathLower == cssOutputPathLower ||
		normalizedFilePathLower == jsOutputPathLower ||
		normalizedFilePathLower == svgOutputPathLower ||
		normalizedFilePathLower == faviconOutputPathLower ||
		normalizedFilePathLower == htmlHandlerOutputPathLower
}
