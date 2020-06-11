package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/function61/deployer/pkg/dstate"
	"github.com/function61/deployer/pkg/githubminiclient"
	"github.com/function61/eventhorizon/pkg/ehreader"
	"github.com/function61/gokit/atomicfilewrite"
	"github.com/function61/gokit/fileexists"
	"github.com/function61/gokit/ossignal"
	"github.com/scylladb/termtables"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func listReleases(ctx context.Context) error {
	app, err := mkApp(ctx)
	if err != nil {
		return err
	}

	releasesTbl := termtables.CreateTable()
	releasesTbl.AddHeaders("Time", "Repo", "Ver", "Id", "Artefact location")

	for _, release := range app.State.AllNewestFirst() {
		releasesTbl.AddRow(
			release.Created.Local().Format("Jan 02 @ 15:04"),
			release.Repository,
			release.RevisionFriendly,
			release.Id,
			release.ArtefactsLocation)
	}

	fmt.Println(releasesTbl.Render())

	return nil
}

func resolveReleaseArtefactsLocationAndDeployerSpecFilename(
	ctx context.Context,
	releaseId string,
) (string, string, error) {
	app, err := mkApp(ctx)
	if err != nil {
		return "", "", err
	}

	release, err := app.State.ById(releaseId)
	if err != nil {
		return "", "", err
	}

	return release.ArtefactsLocation, release.DeployerSpecFilename, nil
}

func downloadRelease(ctx context.Context, serviceId string, releaseId string) error {
	if strings.Contains(releaseId, ":") {
		// expecting file:#deployerspec.zip
		// expecting http://example.com/files/#deployerspec.zip
		parts := strings.Split(releaseId, "#")
		if len(parts) != 2 {
			return fmt.Errorf("don't know how to do hash-less parsing yet: %s", releaseId)
		}

		return downloadReleaseWith(ctx, serviceId, parts[0], parts[1])
	} else {
		artefactsLocation, deployerSpecFilename, err := resolveReleaseArtefactsLocationAndDeployerSpecFilename(
			ctx,
			releaseId)
		if err != nil {
			return fmt.Errorf("resolveReleaseArtefactsLocationAndDeployerSpecFilename: %w", err)
		}

		return downloadReleaseWith(ctx, serviceId, artefactsLocation, deployerSpecFilename)
	}
}

func downloadReleaseWith(
	ctx context.Context,
	serviceId string,
	artefactsLocation string,
	deployerSpecFilename string,
) error {
	// each unique release has different artefactsLocation, so instead of using releaseId
	// hash the location to remove dependency to release ID (so we can deploy manually
	// for testing/dev purposes)
	approxReleaseId := fmt.Sprintf("%x", sha1.Sum([]byte(deployerSpecFilename)))

	allDownloadedFlagPath := filepath.Join(
		workDir(serviceId),
		fmt.Sprintf("_all-downloaded.%s.flag", approxReleaseId))

	allDownloaded, err := fileexists.Exists(allDownloadedFlagPath)
	if err != nil {
		return err
	}

	if allDownloaded {
		return nil // nothing to do here :)
	}

	// var gmc *githubminiclient.Client
	ghToken, err := getGitHubToken()
	if err != nil {
		return err
	}

	gmc, err := githubminiclient.New(githubminiclient.AccessToken(ghToken))
	if err != nil {
		return err
	}

	log.Printf("artefacts source: %s", artefactsLocation)

	artefacts, err := makeArtefactDownloader(artefactsLocation, gmc)
	if err != nil {
		return err
	}

	logDownload := func(filename string) {
		log.Printf("downloading %s", filename)
	}

	logDownload(deployerSpecFilename)

	deployerSpecReader, err := artefacts.DownloadArtefact(ctx, deployerSpecFilename)
	if err != nil {
		return err
	}
	defer deployerSpecReader.Close()

	log.Printf("extracting %s", deployerSpecFilename)

	if err := extractSpecFromReader(serviceId, deployerSpecReader); err != nil {
		return err
	}

	downloadOneArtefact := func(filename string) error { // for defers
		localFilename := filepath.Join(workDir(serviceId), filename)

		exists, err := fileexists.Exists(localFilename)
		if err != nil {
			return err
		}

		if exists {
			log.Println("  already downloaded") // we already have log context from previous line
			return nil
		}

		artefactContent, err := artefacts.DownloadArtefact(ctx, filename)
		if err != nil {
			return err
		}
		defer artefactContent.Close()

		return atomicfilewrite.Write(localFilename, func(dest io.Writer) error {
			_, err := io.Copy(dest, artefactContent)
			return err
		})
	}

	vam, err := loadVersionAndManifest(serviceId)
	if err != nil {
		return err
	}

	for _, downloadArtefact := range vam.Manifest.DownloadArtefacts {
		logDownload(downloadArtefact)

		if err := downloadOneArtefact(downloadArtefact); err != nil {
			return err
		}
	}

	return touch(allDownloadedFlagPath)
}

func releasesEntry(logger *log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Subcommands for releases",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List all releases",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			if err := listReleases(
				ossignal.InterruptOrTerminateBackgroundCtx(logger),
			); err != nil {
				panic(err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "githubrelease-mk [owner] [repo] [releaseName] [revisionId] [assetDir]",
		Short: "Create GitHub release",
		Args:  cobra.ExactArgs(5),
		Run: func(_ *cobra.Command, args []string) {
			if err := createGithubRelease(
				ossignal.InterruptOrTerminateBackgroundCtx(logger),
				args[0],
				args[1],
				args[2],
				args[3],
				args[4],
			); err != nil {
				panic(err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "dl [serviceId] [releaseId]",
		Short: "Download release",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			if err := downloadRelease(
				ossignal.InterruptOrTerminateBackgroundCtx(logger),
				args[0],
				args[1],
			); err != nil {
				panic(err)
			}
		},
	})

	return cmd
}

func ownerSlashRepo(repo githubminiclient.RepoRef) string {
	return fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
}

func artefactsLocationGithubReleases(repo githubminiclient.RepoRef, releaseId int64) string {
	return fmt.Sprintf(
		"githubrelease:%s:%s:%d",
		repo.Owner,
		repo.Name,
		releaseId)
}

func touch(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	return file.Close()
}

func mkApp(ctx context.Context) (*dstate.App, error) {
	tenantCtx, err := ehreader.TenantCtxFrom(ehreader.ConfigFromEnv)
	if err != nil {
		return nil, err
	}

	app, err := dstate.LoadUntilRealtime(
		ctx,
		tenantCtx,
		nil)
	if err != nil {
		return nil, err
	}

	return app, nil
}
