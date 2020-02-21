package main

import (
	"context"
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
)

func listReleases(ctx context.Context) error {
	tenantCtx, err := ehreader.TenantConfigFromEnv()
	if err != nil {
		return err
	}

	app, err := dstate.LoadUntilRealtime(
		ctx,
		dstate.New(tenantCtx.Tenant, nil),
		tenantCtx.Client)
	if err != nil {
		return err
	}

	releasesTbl := termtables.CreateTable()
	releasesTbl.AddHeaders("Repo", "Ver", "Id", "Artefact location")

	for _, release := range app.State.All() {
		releasesTbl.AddRow(
			release.Repository,
			release.RevisionFriendly,
			release.Id,
			release.ArtefactsLocation)
	}

	fmt.Println(releasesTbl.Render())

	return nil
}

func downloadRelease(ctx context.Context, serviceId string, releaseId string) error {
	tenantCtx, err := ehreader.TenantConfigFromEnv()
	if err != nil {
		return err
	}

	app, err := dstate.LoadUntilRealtime(
		ctx,
		dstate.New(tenantCtx.Tenant, nil),
		tenantCtx.Client)
	if err != nil {
		return err
	}

	release, err := app.State.ById(releaseId)
	if err != nil {
		return err
	}

	allDownloadedFlagPath := filepath.Join(
		workDir(serviceId),
		fmt.Sprintf("_all-downloaded.%s.flag", releaseId))

	allDownloaded, err := fileexists.Exists(allDownloadedFlagPath)
	if err != nil {
		return err
	}

	if allDownloaded {
		return nil // nothing to do here :)
	}

	ghToken, err := getGitHubToken()
	if err != nil {
		return err
	}

	gmc, err := githubminiclient.New(githubminiclient.AccessToken(ghToken))
	if err != nil {
		return err
	}

	log.Printf("artefacts source: %s", release.ArtefactsLocation)

	artefacts, err := makeArtefactDownloader(release.ArtefactsLocation, gmc)
	if err != nil {
		return err
	}

	logDownload := func(filename string) {
		log.Printf("downloading %s", filename)
	}

	logDownload(release.DeployerSpecFilename)

	deployerSpecReader, err := artefacts.DownloadArtefact(ctx, release.DeployerSpecFilename)
	if err != nil {
		return err
	}
	defer deployerSpecReader.Close()

	log.Printf("extracting %s", release.DeployerSpecFilename)

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
