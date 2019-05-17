package recursivefilecopy

import (
	"io"
	"os"
	"path/filepath"
)

// could not use https://github.com/otiai10/copy becase:
// https://twitter.com/joonas_fi/status/1129319037140459520
func Copy(source string, destination string) error {
	handleOneFile := func(path string, info os.FileInfo) error {
		relSource, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		dstFilename := filepath.Join(destination, relSource)

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(dstFilename), 0755); err != nil {
			return err
		}

		dstFile, err := os.Create(dstFilename)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err = io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		return dstFile.Chmod(info.Mode())
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		return handleOneFile(path, info)
	})
}
