package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/function61/gokit/jsonfile"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// same as extractSpec, but hides stupid buffering stuff required by io.ReaderAt
func extractSpecFromReader(serviceId string, zipFile io.Reader) error {
	buf := &bytes.Buffer{}

	if _, err := io.Copy(buf, zipFile); err != nil {
		return err
	}

	bufReader := bytes.NewReader(buf.Bytes())

	return extractSpec(serviceId, bufReader, int64(bufReader.Len()))
}

func extractSpec(serviceId string, zipFile io.ReaderAt, size int64) error {
	zipReader, err := zip.NewReader(zipFile, size)
	if err != nil {
		return err
	}

	extractOneFile := func(f *zip.File) error {
		if f.FileInfo().IsDir() { // skip dirs. NOTE: laziness means that empty dirs will not be created
			return nil
		}

		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("file %s in zip tries to exploit path traversal", f.Name)
		}

		path := workDir(serviceId) + "/" + f.Name

		dirOfPath := filepath.Dir(path)

		if err := os.MkdirAll(dirOfPath, 0755); err != nil {
			return err
		}

		fsFile, err := os.Create(path)
		if err != nil {
			return err
		}

		zipFileReader, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(fsFile, zipFileReader)

		return err
	}

	for _, file := range zipReader.File {
		if err := extractOneFile(file); err != nil {
			return err
		}
	}

	return nil
}

func loadVersionAndManifest(serviceId string) (*VersionAndManifest, error) {
	manifest, err := readAndValidateManifest(workDir(serviceId))
	if err != nil {
		return nil, err
	}

	version := &Version{}
	if err := jsonfile.Read(workDir(serviceId)+"/"+versionJsonFilename, version, true); err != nil {
		return nil, err
	}

	return &VersionAndManifest{
		Version:  *version,
		Manifest: *manifest,
	}, nil
}

func readAndValidateManifest(dir string) (*DeplSpecManifest, error) {
	manifest := &DeplSpecManifest{}
	if err := jsonfile.Read(dir+"/manifest.json", manifest, true); err != nil {
		return nil, err
	}

	if manifest.ManifestVersionMajor != 1 {
		return nil, fmt.Errorf("unsupported manifest version; got %d", manifest.ManifestVersionMajor)
	}

	return manifest, nil
}
