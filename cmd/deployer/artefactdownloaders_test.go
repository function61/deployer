package main

import (
	"context"
	"testing"

	"github.com/function61/gokit/assert"
)

func TestGithubReleases(t *testing.T) {
	downloader, err := makeArtefactDownloader(context.TODO(), "githubrelease:function61:coolproduct:12345", nil)

	assert.Ok(t, err)

	ghr := downloader.(*githubReleasesArtefactDownloader)

	assert.EqualString(t, ghr.owner, "function61")
	assert.EqualString(t, ghr.repo, "coolproduct")
	assert.Assert(t, ghr.releaseId == 12345)
}

func TestHttp(t *testing.T) {
	downloader, err := makeArtefactDownloader(context.TODO(), "http://downloads.example.com/", nil)

	assert.Ok(t, err)

	had := downloader.(*httpArtefactDownloader)

	assert.EqualString(t, had.baseUrl, "http://downloads.example.com/")
}

func TestHttps(t *testing.T) {
	downloader, err := makeArtefactDownloader(context.TODO(), "https://downloads.example.com/", nil)

	assert.Ok(t, err)

	had := downloader.(*httpArtefactDownloader)

	assert.EqualString(t, had.baseUrl, "https://downloads.example.com/")
}

func TestUnsupportedUri(t *testing.T) {
	_, err := makeArtefactDownloader(context.TODO(), "ftp://stuff", nil)

	assert.EqualString(t, err.Error(), "unsupported URI: ftp://stuff")
}
