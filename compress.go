package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

func compressFile(data []byte, filename string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	fileWriter, err := zipWriter.Create(filename)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(fileWriter, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	return &buf, nil
}

func ExtractFileFromZipToBytes(zipFilename string) (string, []byte, error) {
	// Open the zip file for reading
	zipReader, err := zip.OpenReader(zipFilename)
	if err != nil {
		return "", nil, err
	}
	defer zipReader.Close()

	// Check if the zip archive contains only one file
	if len(zipReader.File) != 1 {
		return "", nil, fmt.Errorf("zip archive must contain exactly one file")
	}

	// Get the first file in the zip archive
	zipFile := zipReader.File[0]

	// Open the file for reading
	fileReader, err := zipFile.Open()
	if err != nil {
		return "", nil, err
	}
	defer fileReader.Close()

	// Read the file contents into a byte buffer
	var fileContentsBuffer bytes.Buffer
	_, err = io.Copy(&fileContentsBuffer, fileReader)
	if err != nil {
		return "", nil, err
	}

	// Return the original filename and the file contents as a byte slice
	fmt.Println("original filename:", zipFile.Name)
	return zipFile.Name, fileContentsBuffer.Bytes(), nil
}
