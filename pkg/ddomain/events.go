// Structure of data for all state changes
package ddomain

import (
	"github.com/function61/eventhorizon/pkg/ehevent"
)

var Types = ehevent.Allocators{
	"ReleaseCreated": func() ehevent.Event { return &ReleaseCreated{} },
}

// ------

type ReleaseCreated struct {
	meta                 ehevent.EventMeta
	Id                   string
	Repository           string
	RevisionFriendly     string
	RevisionId           string
	ArtefactsLocation    string // "https://baseurl" if easy to download. or "githubrelease:owner:reponame:releaseId"
	DeployerSpecFilename string // usually "deployerspec.zip"
}

func (e *ReleaseCreated) MetaType() string         { return "ReleaseCreated" }
func (e *ReleaseCreated) Meta() *ehevent.EventMeta { return &e.meta }

func NewReleaseCreated(
	id string,
	repository string,
	revisionFriendly string,
	revisionId string,
	artefactsLocation string,
	deployerSpecFilename string,
	meta ehevent.EventMeta,
) *ReleaseCreated {
	return &ReleaseCreated{
		meta:                 meta,
		Id:                   id,
		Repository:           repository,
		RevisionFriendly:     revisionFriendly,
		RevisionId:           revisionId,
		ArtefactsLocation:    artefactsLocation,
		DeployerSpecFilename: deployerSpecFilename,
	}
}
