package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/function61/deployer/pkg/ddomain"
	"github.com/function61/deployer/pkg/dstate"
	"github.com/function61/deployer/pkg/githubminiclient"
	"github.com/function61/eventhorizon/pkg/ehevent"
	"github.com/function61/gokit/backoff"
	"github.com/function61/gokit/cryptorandombytes"
	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/retry"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

func createGithubRelease(
	ctx context.Context,
	owner string,
	repoName string,
	releaseName string,
	revisionId string,
	assetsDir string,
) error {
	repo := githubminiclient.NewRepoRef(owner, repoName)

	ghToken, err := getGitHubToken()
	if err != nil {
		return err
	}

	gitHub := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: ghToken,
	})))

	// search for existing releases, because if artefact uploading fails and we've
	// to re-run this again, we don't want to end up with same release name twice
	// FIXME: this doesn't take pagination into account
	existingReleases, _, err := gitHub.Repositories.ListReleases(ctx, repo.Owner, repo.Name, nil)
	if err != nil {
		return err
	}

	var releaseID int64

	for _, existingRelease := range existingReleases {
		if *existingRelease.Name == releaseName {
			releaseID = *existingRelease.ID
			break
		}
	}

	// release not found => create one
	if releaseID == 0 {
		createdRelease, _, err := gitHub.Repositories.CreateRelease(ctx, repo.Owner, repo.Name, &github.RepositoryRelease{
			Name:            github.String(releaseName),
			TagName:         github.String(releaseName),
			TargetCommitish: github.String(revisionId),
			Draft:           github.Bool(true),
		})
		if err != nil {
			return err
		}

		releaseID = *createdRelease.ID
	}

	if os.Getenv("EVENTHORIZON") == "" { // only notify Event Horizon if we it configured
		return nil
	}

	app, err := mkApp(ctx)
	if err != nil {
		return err
	}

	if app.State.HasRevisionId(revisionId) {
		return fmt.Errorf("already have revision %s", revisionId)
	}

	if err := uploadArtefacts(ctx, assetsDir, repo, releaseID, gitHub); err != nil {
		return err
	}

	releaseCreated := ddomain.NewReleaseCreated(
		cryptorandombytes.Base64UrlWithoutLeadingDash(4),
		ownerSlashRepo(repo), // function61/coolproduct
		releaseName,
		revisionId,
		artefactsLocationGithubReleases(repo, releaseID),
		"deployerspec.zip",                 // TODO: this shouldn't be hardcoded
		ehevent.MetaSystemUser(time.Now())) // TODO: time of commit?

	_, err = app.Writer.Append(
		ctx,
		app.TenantCtx.Tenant.Stream(dstate.Stream),
		[]string{ehevent.Serialize(releaseCreated)})
	return err
}

func uploadArtefacts(
	ctx context.Context,
	assetsDir string,
	repo githubminiclient.RepoRef,
	releaseID int64,
	gitHub *github.Client,
) error {
	startUpload := make(chan string)

	uploaders, uploadersCtx := concurrently(ctx, 3, func(ctx context.Context) error {
		for filePath := range startUpload {
			if err := uploadOneArtefact(ctx, filePath, releaseID, gitHub, repo); err != nil {
				return err
			}
		}

		return nil
	})

	dentries, err := ioutil.ReadDir(assetsDir)
	if err != nil {
		return err
	}

	// func b/c we need return keyword
	func() {
		for _, dentry := range dentries {
			filePath := filepath.Join(assetsDir, dentry.Name())

			select {
			case startUpload <- filePath:
			case <-uploadersCtx.Done():
				// uploaders aborted - stop submitting work. Wait() will return the error
				return // cannot "break" here because it'd only break out of the select
			}
		}
	}()

	// signal that there will be no more jobs - workers will exit after processing their
	// last job.
	close(startUpload)

	// errors if any of the uploads errored
	if err := uploaders.Wait(); err != nil {
		return err
	}

	return nil
}

func uploadOneArtefact(
	ctx context.Context,
	filePath string,
	releaseID int64,
	gh *github.Client,
	repo githubminiclient.RepoRef,
) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// I have observed GitHub asset uploads to regularly fail (even from GitHub runners)
	return retry.Retry(ctx, func(ctx context.Context) error {
		log.Printf("uploading %s", filePath)

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, res, err := gh.Repositories.UploadReleaseAsset(ctx, repo.Owner, repo.Name, releaseID, &github.UploadOptions{
			Name: filepath.Base(filePath),
		}, file)

		if err != nil {
			// I witnessed GitHub receive artefact but it still errored to us, so be more
			// robust against these conditions
			if res != nil && res.StatusCode == http.StatusUnprocessableEntity {
				// 422 Validation Failed [{Resource:ReleaseAsset Field:name Code:already_exists Message:}]
				return nil // was already uploaded so essentially not an error
			} else {
				return err
			}
		}

		return nil
	}, backoff.ExponentialWithCappedMax(1*time.Second, 15*time.Second), func(err error) {
		log.Printf("upload %s try failed: %v", filePath, err)
	})
}

func concurrently(
	ctx context.Context,
	concurrency int,
	task func(ctx context.Context) error,
) (*errgroup.Group, context.Context) {
	// if any of the workers error, taskCtx will be canceled
	group, taskCtx := errgroup.WithContext(ctx)

	for i := 0; i < concurrency; i++ {
		group.Go(func() error {
			return task(taskCtx)
		})
	}

	return group, taskCtx
}

func getGitHubToken() (string, error) {
	return envvar.Required("GITHUB_TOKEN")
}
