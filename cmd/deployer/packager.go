package main

import (
	"archive/zip"
	"github.com/function61/gokit/jsonfile"
	"io"
	"os"
	"path/filepath"
)

func makePackage(friendlyVersion string, outputFile string) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	zipWriter := zip.NewWriter(f)
	defer zipWriter.Close()

	versionFile, err := zipWriter.Create(versionJsonFilename)
	if err != nil {
		return err
	}

	if err := jsonfile.Marshal(versionFile, &Version{FriendlyVersion: friendlyVersion}); err != nil {
		return nil
	}

	// TODO: validate manifest

	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		entryInZip, err := zipWriter.Create(path)
		if err != nil {
			return err
		}

		oFile, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(entryInZip, oFile); err != nil {
			oFile.Close()
			return err
		}

		return oFile.Close()
	})
}
