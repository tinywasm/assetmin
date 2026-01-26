package assetmin

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// pathFile e.g., "theme/htmlMainFileName"
// data e.g., *bytes.Buffer
// NOTE: The buffer data will be cleared after writing the file
func FileWrite(pathFile string, data bytes.Buffer) error {
	const e = "FileWrite "

	// Ensure the directory exists before creating the file
	dir := filepath.Dir(pathFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.New(e + "while creating directory " + err.Error())
	}

	dst, err := os.Create(pathFile)
	if err != nil {
		return errors.New(e + "while creating file " + err.Error())
	}
	defer dst.Close()

	// Copy the uploaded contentFile to the filesystem at the specified destination
	_, err = io.Copy(dst, &data)
	if err != nil {
		return errors.New(e + "failed to write the file " + pathFile + " to the destination " + err.Error())
	}

	return nil
}

// FileWriteSafe writes the data to pathFile only if the file does not already exist.
func FileWriteSafe(pathFile string, data bytes.Buffer) error {
	const e = "FileWriteSafe "

	// Check if the file already exists
	if _, err := os.Stat(pathFile); err == nil {
		// File already exists, skip writing
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return errors.New(e + "while checking file existence " + err.Error())
	}

	return FileWrite(pathFile, data)
}
