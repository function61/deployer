// GitHub mini client - mainly to resolve and download assets
package githubminiclient

import (
	"context"
	"fmt"
	"io"

	"github.com/function61/gokit/ezhttp"
)

const (
	DefaultEndpoint = "https://api.github.com"
)

type RepoRef struct {
	Owner string
	Name  string
}

func NewRepoRef(user, name string) RepoRef {
	return RepoRef{user, name}
}

type Asset struct {
	Id   uint64 `json:"id"`
	Name string `json:"name"` // filename (does not contain path)
	Url  string `json:"url"`  // to download the asset
}

type Client struct {
	personalAccessToken string
}

type accessTokenObtainer func() (string, error)

func New(tokenFn accessTokenObtainer) (*Client, error) {
	personalAccessToken, err := tokenFn()
	if err != nil {
		return nil, err
	}

	return &Client{personalAccessToken}, nil
}

func (g *Client) ListAssetsForRelease(ctx context.Context, repo RepoRef, releaseId string) ([]Asset, error) {
	// https://stackoverflow.com/questions/20396329/how-to-download-github-release-from-private-repo-using-command-line
	// NOTE: there's also "release by tag" endpoint available
	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s/releases/%s",
		DefaultEndpoint,
		repo.Owner,
		repo.Name,
		releaseId)

	resp := struct {
		Assets []Asset `json:"assets"`
	}{}

	if _, err := ezhttp.Get(
		ctx,
		endpoint,
		ezhttp.Header("Authorization", "token "+g.personalAccessToken),
		ezhttp.RespondsJson(&resp, true),
	); err != nil {
		return nil, err
	}

	return resp.Assets, nil
}

func (g *Client) DownloadAsset(ctx context.Context, asset Asset) (io.ReadCloser, error) {
	resp, err := ezhttp.Get(
		ctx,
		asset.Url,
		ezhttp.Header("Accept", "application/octet-stream"),
		ezhttp.Header("Authorization", "token "+g.personalAccessToken),
	)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func AccessToken(tok string) accessTokenObtainer {
	return func() (string, error) {
		return tok, nil
	}
}
