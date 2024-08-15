package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/function61/deployer/pkg/oci"
	"github.com/function61/gokit/jsonfile"
)

type ociArtefactDownloader struct {
	imageRef           string
	imageRefWithoutTag string
	manifest           oci.Manifest
}

func newOCIArtefactDownloader(ctx context.Context, imageRef string) (artefactDownloader, error) {
	withErr := func(err error) (artefactDownloader, error) {
		return nil, fmt.Errorf("newOCIArtefactDownloader: %w", err)
	}

	orasFetchOutput := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	orasFetch := exec.CommandContext(ctx, "oras", "manifest", "fetch", imageRef)
	orasFetch.Stdout = orasFetchOutput
	orasFetch.Stderr = stderr

	if err := orasFetch.Run(); err != nil {
		return withErr(fmt.Errorf("oras manifest fetch %s: %w: stdout[%s] stderr[%s]", imageRef, err, orasFetchOutput.String(), stderr.String()))
	}

	manifest := oci.Manifest{}
	if err := jsonfile.Unmarshal(orasFetchOutput, &manifest, false); err != nil {
		return withErr(err)
	}

	// "redis:latest" => ["redis", "latest"]
	imageRefWithoutTag := strings.Split(imageRef, ":")
	if len(imageRefWithoutTag) != 2 {
		return withErr(fmt.Errorf("expected imageRef to be <something>:<tag>; got %s", imageRef))
	}

	return &ociArtefactDownloader{imageRef, imageRefWithoutTag[0], manifest}, nil
}

func (o *ociArtefactDownloader) DownloadArtefact(ctx context.Context, filename string) (io.ReadCloser, error) {
	withErr := func(err error) (io.ReadCloser, error) {
		return nil, fmt.Errorf("ociArtefactDownloader.DownloadArtefact: %w", err)
	}

	layer, found := func() (*oci.Layer, bool) {
		for _, layer := range o.manifest.Layers {
			if layer.Annotations["org.opencontainers.image.title"] == filename {
				return &layer, true
			}
		}

		return nil, false
	}()
	if !found {
		return withErr(fmt.Errorf("%s not found from manifest of %s", filename, o.imageRef))
	}

	// "redis" => "redis@sha256:..."
	blobRef := o.imageRefWithoutTag + "@" + layer.Digest

	// blobReader, blobWriter := io.Pipe()

	blobReader := &bytes.Buffer{}

	orasBlobFetch := exec.CommandContext(ctx, "oras", "blob", "fetch", "--output=-", blobRef)
	orasBlobFetch.Stdout = blobReader
	// orasBlobFetch.Stdout = blobWriter

	if err := orasBlobFetch.Run(); err != nil {
		return withErr(err)
	}

	return io.NopCloser(blobReader), nil
}
