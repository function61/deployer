package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/function61/deployer/pkg/githubminiclient"
	"github.com/function61/gokit/ezhttp"
)

type artefactDownloader interface {
	DownloadArtefact(ctx context.Context, filename string) (io.ReadCloser, error)
}

type githubReleasesArtefactDownloader struct {
	owner        string
	repo         string
	releaseId    int64
	gmc          *githubminiclient.Client
	mu           sync.Mutex
	cachedAssets []githubminiclient.Asset
}

func newGithubReleasesArtefactDownloader(uri string, gmc *githubminiclient.Client) (*githubReleasesArtefactDownloader, error) {
	components := strings.Split(uri, ":")
	if len(components) != 4 {
		return nil, fmt.Errorf(
			"invalid syntax for githubReleasesArtefactDownloader, got %d components",
			len(components))
	}

	if components[0] != "githubrelease" {
		return nil, fmt.Errorf("expecting githubrelease; got '%s'", components[0])
	}

	releaseId, err := strconv.Atoi(components[3])
	if err != nil {
		return nil, err
	}

	return &githubReleasesArtefactDownloader{
		owner:     components[1],
		repo:      components[2],
		releaseId: int64(releaseId),
		gmc:       gmc,
	}, nil
}

func (g *githubReleasesArtefactDownloader) DownloadArtefact(
	ctx context.Context,
	filename string,
) (io.ReadCloser, error) {
	if err := g.downloadAndCacheAssetMetadata(ctx); err != nil {
		return nil, err
	}

	for _, asset := range g.cachedAssets {
		if asset.Name == filename {
			return g.gmc.DownloadAsset(ctx, asset)
		}
	}

	return nil, fmt.Errorf("asset to download not found: %s", filename)
}

func (g *githubReleasesArtefactDownloader) downloadAndCacheAssetMetadata(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cachedAssets != nil {
		return nil
	}

	repo := githubminiclient.NewRepoRef(g.owner, g.repo)

	assets, err := g.gmc.ListAssetsForRelease(ctx, repo, strconv.Itoa(int(g.releaseId)))
	if err != nil {
		return err
	}

	g.cachedAssets = assets

	return nil
}

type httpArtefactDownloader struct {
	baseUrl string
}

func newhttpArtefactDownloader(uri string) (*httpArtefactDownloader, error) {
	return &httpArtefactDownloader{uri}, nil
}

func (h *httpArtefactDownloader) DownloadArtefact(
	ctx context.Context,
	filename string,
) (io.ReadCloser, error) {
	res, err := ezhttp.Get(ctx, h.baseUrl+filename)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}

// in practice makes a copy of a local file
type localFileDownloader struct {
	path string
}

func newLocalFileDownloader(path string) artefactDownloader {
	return &localFileDownloader{path}
}

func (f *localFileDownloader) DownloadArtefact(ctx context.Context, filename string) (io.ReadCloser, error) {
	file, err := os.Open(filepath.Join(f.path, filename))
	if err != nil {
		return nil, err
	}

	return file, nil
}

func makeArtefactDownloader(uri string, gmc *githubminiclient.Client) (artefactDownloader, error) {
	switch {
	case strings.HasPrefix(uri, "file:"):
		return newLocalFileDownloader(uri[len("file:"):]), nil
	case strings.HasPrefix(uri, "http:"), strings.HasPrefix(uri, "https:"):
		return newhttpArtefactDownloader(uri)
	case strings.HasPrefix(uri, "githubrelease:"):
		return newGithubReleasesArtefactDownloader(uri, gmc)
	default:
		return nil, fmt.Errorf("unsupported URI: %s", uri)
	}
}
