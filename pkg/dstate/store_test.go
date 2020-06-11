package dstate

import (
	"context"
	"testing"
	"time"

	"github.com/function61/deployer/pkg/ddomain"
	"github.com/function61/eventhorizon/pkg/ehevent"
	"github.com/function61/eventhorizon/pkg/ehreader"
	"github.com/function61/eventhorizon/pkg/ehreader/ehreadertest"
	"github.com/function61/gokit/assert"
)

func TestStore(t *testing.T) {
	t0 := time.Date(2020, 2, 20, 14, 2, 0, 0, time.UTC)

	eventLog := ehreadertest.NewEventLog()
	eventLog.AppendE(
		"/t-42/software-releases",
		ddomain.NewReleaseCreated(
			"id1",
			"function61/coolproduct",
			"20200219_1609_9c39d027",
			"9c39d0271d0bd51c7ddfb55dc3051e68b6953c33",
			"https://download.com/dl/",
			"deployerspec.zip",
			ehevent.MetaSystemUser(t0)),
	)

	app, err := LoadUntilRealtime(
		context.Background(),
		ehreader.NewTenantCtx(ehreader.TenantId("42"), eventLog),
		nil)
	assert.Ok(t, err)

	releases := app.State.All()

	assert.Assert(t, len(releases) == 1)
	assert.EqualString(t, releases[0].Id, "id1")
	assert.EqualString(t, releases[0].Repository, "function61/coolproduct")
	assert.EqualString(t, releases[0].RevisionFriendly, "20200219_1609_9c39d027")
	assert.EqualString(t, releases[0].RevisionId, "9c39d0271d0bd51c7ddfb55dc3051e68b6953c33")
	assert.EqualString(t, releases[0].ArtefactsLocation, "https://download.com/dl/")
	assert.EqualString(t, releases[0].DeployerSpecFilename, "deployerspec.zip")
}
