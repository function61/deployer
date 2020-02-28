package dstate

import (
	"context"
	"fmt"
	"github.com/function61/deployer/pkg/ddomain"
	"github.com/function61/eventhorizon/pkg/ehclient"
	"github.com/function61/eventhorizon/pkg/ehevent"
	"github.com/function61/eventhorizon/pkg/ehreader"
	"github.com/function61/gokit/logex"
	"log"
	"sync"
	"time"
)

type SoftwareRelease struct {
	Id                   string
	Created              time.Time
	Repository           string
	RevisionFriendly     string
	RevisionId           string
	ArtefactsLocation    string
	DeployerSpecFilename string
}

const (
	Stream = "/software-releases"
)

type Store struct {
	version  ehclient.Cursor
	mu       sync.Mutex
	releases []SoftwareRelease
	logl     *logex.Leveled
}

func New(tenant ehreader.Tenant, logger *log.Logger) *Store {
	return &Store{
		version:  ehclient.Beginning(tenant.Stream(Stream)),
		releases: []SoftwareRelease{},
		logl:     logex.Levels(logger),
	}
}

func (c *Store) ById(releaseId string) (*SoftwareRelease, error) {
	for _, release := range c.All() { // uses locking
		if release.Id == releaseId {
			return &release, nil
		}
	}

	return nil, fmt.Errorf("Release not found by ID: %s", releaseId)
}

// negligible chance of collisions across repos
func (c *Store) HasRevisionId(revisionId string) bool {
	for _, release := range c.All() { // uses locking
		if release.RevisionId == revisionId {
			return true
		}
	}

	return false
}

func (c *Store) Version() ehclient.Cursor {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.version
}

func (c *Store) AllNewestFirst() []SoftwareRelease {
	all := c.All()
	// FFS there's no generic slice reverse in Go..
	count := len(all)
	reversed := make([]SoftwareRelease, count)
	for i := 0; i < count; i++ {
		reversed[i] = all[count-1-i]
	}
	return reversed
}

func (c *Store) All() []SoftwareRelease {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.releases
}

func (c *Store) GetEventTypes() ehevent.Allocators {
	return ddomain.Types
}

func (c *Store) ProcessEvents(_ context.Context, processAndCommit ehreader.EventProcessorHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return processAndCommit(
		c.version,
		func(ev ehevent.Event) error { return c.processEvent(ev) },
		func(version ehclient.Cursor) error {
			c.version = version
			return nil
		})
}

func (c *Store) processEvent(ev ehevent.Event) error {
	c.logl.Info.Println(ev.MetaType())

	switch e := ev.(type) {
	case *ddomain.ReleaseCreated:
		c.releases = append(c.releases, SoftwareRelease{
			Id:                   e.Id,
			Created:              e.Meta().Timestamp,
			Repository:           e.Repository,
			RevisionFriendly:     e.RevisionFriendly,
			RevisionId:           e.RevisionId,
			ArtefactsLocation:    e.ArtefactsLocation,
			DeployerSpecFilename: e.DeployerSpecFilename,
		})
	default:
		return ehreader.UnsupportedEventTypeErr(ev)
	}

	return nil
}

type App struct {
	State     *Store
	Reader    *ehreader.Reader
	Writer    ehclient.Writer
	TenantCtx *ehreader.TenantCtx
}

func LoadUntilRealtime(
	ctx context.Context,
	tenantCtx *ehreader.TenantCtx,
	logger *log.Logger,
) (*App, error) {
	store := New(tenantCtx.Tenant, logger)

	a := &App{
		store,
		ehreader.New(
			store,
			tenantCtx.Client,
			logger),
		tenantCtx.Client,
		tenantCtx}

	if err := a.Reader.LoadUntilRealtime(ctx); err != nil {
		return nil, err
	}

	return a, nil
}
