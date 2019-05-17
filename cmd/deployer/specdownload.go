package main

import (
	"archive/zip"
	"context"
	"fmt"
	"github.com/function61/deployer/pkg/tempfile"
	"github.com/function61/gokit/ezhttp"
	"github.com/function61/gokit/jsonfile"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func downloadAndExtractSpecByUrl(serviceId string, url string) (*VersionAndManifest, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), ezhttp.DefaultTimeout10s)
	defer cancel()

	tempFile, cleanupTempFile, err := tempfile.New("deployer")
	if err != nil {
		return nil, err
	}
	defer cleanupTempFile()

	res, err := ezhttp.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if _, err := io.Copy(tempFile, res.Body); err != nil {
		return nil, err
	}

	return extractSpec(serviceId, tempFile.Name())
}

func extractSpec(serviceId string, pathToZip string) (*VersionAndManifest, error) {
	zipFile, err := os.Open(pathToZip)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()

	zipStat, err := zipFile.Stat()
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(zipFile, zipStat.Size())
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}

	manifest := &DeplSpecManifest{}
	if err := jsonfile.Read(workDir(serviceId)+"/manifest.json", manifest, true); err != nil {
		return nil, err
	}

	if manifest.ManifestVersionMajor != 1 {
		return nil, fmt.Errorf("unsupported manifest version; got %d", manifest.ManifestVersionMajor)
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

func downloadArtefacts(serviceId string, vam VersionAndManifest) error {
	downloadOne := func(artefactFilename string) error {
		artefactUrl := strings.Replace(
			strings.Replace(
				vam.Manifest.DownloadArtefactUrlTemplate,
				"{version}",
				vam.Version.FriendlyVersion, -1),
			"{filename}",
			artefactFilename,
			-1)

		ctx, cancel := context.WithTimeout(context.TODO(), ezhttp.DefaultTimeout10s)
		defer cancel()

		resp, err := ezhttp.Get(ctx, artefactUrl)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		file, err := os.Create(workDir(serviceId) + "/" + artefactFilename)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			return err
		}

		return file.Close()
	}

	for _, artefactFilename := range vam.Manifest.DownloadArtefacts {
		if err := downloadOne(artefactFilename); err != nil {
			return err
		}
	}

	return nil
}
