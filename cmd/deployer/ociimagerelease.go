package main

// OCI image releaser

import (
	"context"
	"log"
	"time"

	"github.com/function61/deployer/pkg/ddomain"
	"github.com/function61/deployer/pkg/dstate"
	"github.com/function61/deployer/pkg/githubminiclient"
	"github.com/function61/eventhorizon/pkg/ehevent"
	"github.com/function61/gokit/cryptorandombytes"
)

func createOCIImageRelease(
	ctx context.Context,
	imageRef string,
	owner string,
	repoName string,
	releaseName string,
	revisionId string,
	logger *log.Logger,
) error {
	repo := githubminiclient.NewRepoRef(owner, repoName)

	app, err := mkApp(ctx)
	if err != nil {
		return err
	}

	if app.State.HasRevisionId(revisionId) { // should not be considered an error
		logger.Printf("WARN: already have revision %s", revisionId)
		return nil
	}

	releaseCreated := ddomain.NewReleaseCreated(
		cryptorandombytes.Base64UrlWithoutLeadingDash(4),
		ownerSlashRepo(repo), // function61/coolproduct
		releaseName,
		revisionId,
		"docker://"+imageRef,
		"",
		ehevent.MetaSystemUser(time.Now())) // TODO: time of commit?

	_, err = app.Writer.Append(
		ctx,
		app.TenantCtx.Tenant.Stream(dstate.Stream),
		[]string{ehevent.Serialize(releaseCreated)})
	return err
}
