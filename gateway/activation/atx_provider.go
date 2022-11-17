package activation

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
	"github.com/spacemeshos/smutil/log"

	"github.com/spacemeshos/poet/shared"
	"github.com/spacemeshos/poet/types"
)

// roundRobinAtxProvider gathers many AtxProviders.
// It tries to get an ATX from them in a round robin fashion.
type roundRobinAtxProvider struct {
	services    []types.AtxProvider
	lastUsedSvc int
	log         log.Log
}

func (a *roundRobinAtxProvider) Get(ctx context.Context, id shared.ATXID) (*types.ATX, error) {
	for retries := 0; retries < len(a.services); retries++ {
		if atx, err := a.services[a.lastUsedSvc].Get(ctx, id); err == nil {
			return atx, nil
		}
		a.lastUsedSvc = (a.lastUsedSvc + 1) % len(a.services)
	}

	return nil, &AtxNotFoundError{id}
}

func NewRoundRobinAtxProvider(services []types.AtxProvider) types.AtxProvider {
	return &roundRobinAtxProvider{
		log:      log.NewDefault("atx_store"),
		services: services,
	}
}

// cachingAtxProvider implements caching layer on top of
// its AtxProvider.
type cachingAtxProvider struct {
	cache   *lru.Cache
	fetcher types.AtxProvider
	log     log.Log
}

func (a *cachingAtxProvider) Get(ctx context.Context, id shared.ATXID) (*types.ATX, error) {
	logger := a.log.WithFields(log.ByteString("id", id[:]))
	if atx, ok := a.cache.Get(id); ok {
		logger.Debug("retrieved ATX from the cache")
		// SAFETY: type assertion will never panic as we insert only `*ATX` values.
		return atx.(*types.ATX), nil
	}

	logger.Debug("fetching ATX from gateways")
	atx, err := a.fetcher.Get(ctx, id)
	if err == nil {
		a.cache.Add(id, atx)
	}
	return atx, err
}

func NewCachedAtxProvider(size int, fetcher types.AtxProvider) (types.AtxProvider, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &cachingAtxProvider{
		log:     log.NewDefault("atx_store"),
		cache:   cache,
		fetcher: fetcher,
	}, nil
}
