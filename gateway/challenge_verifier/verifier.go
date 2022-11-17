package challenge_verifier

import (
	"context"
	"crypto/sha256"
	"errors"

	lru "github.com/hashicorp/golang-lru"
	"github.com/spacemeshos/smutil/log"
	"go.uber.org/zap"

	"github.com/spacemeshos/poet/logging"
	"github.com/spacemeshos/poet/types"
)

// roundRobinChallengeVerifier gathers many verifiers.
// It tries to verify a challenge in a round robin fashion,
// retrying with the next verifier if the previous one was not
// able to complete verification.
type roundRobinChallengeVerifier struct {
	services    []types.ChallengeVerifier
	lastUsedSvc int
}

func (a *roundRobinChallengeVerifier) Verify(ctx context.Context, challenge []byte, signature []byte) ([]byte, error) {
	for retries := 0; retries < len(a.services); retries++ {
		hash, err := a.services[a.lastUsedSvc].Verify(ctx, challenge, signature)
		if err == nil {
			return hash, nil
		}
		if errors.Is(err, types.ErrChallengeInvalid) {
			return nil, err
		}
		a.lastUsedSvc = (a.lastUsedSvc + 1) % len(a.services)
	}

	return nil, types.ErrCouldNotVerify
}

func NewRoundRobinChallengeVerifier(services []types.ChallengeVerifier) types.ChallengeVerifier {
	return &roundRobinChallengeVerifier{
		services: services,
	}
}

// cachingChallengeVerifier implements caching layer on top of
// its ChallengeVerifier.
type cachingChallengeVerifier struct {
	cache    *lru.Cache
	verifier types.ChallengeVerifier
}

type challengeVerifierResult struct {
	hash []byte
	err  error
}

func (a *cachingChallengeVerifier) Verify(ctx context.Context, challenge []byte, signature []byte) ([]byte, error) {
	var challengeHash [sha256.Size]byte
	hasher := sha256.New()
	hasher.Write(challenge)
	hasher.Write(signature)
	hasher.Sum(challengeHash[:0])

	logger := logging.FromContext(ctx).WithFields(log.Field(zap.Binary("challenge", challengeHash[:])))
	if result, ok := a.cache.Get(challengeHash); ok {
		logger.Debug("retrieved challenge verifier result from the cache")
		// SAFETY: type assertion will never panic as we insert only `*ATX` values.
		result := result.(*challengeVerifierResult)
		return result.hash, result.err
	}

	hash, err := a.verifier.Verify(ctx, challenge, signature)
	if err == nil || errors.Is(err, types.ErrChallengeInvalid) {
		logger.With().Debug("finished challenge verification", log.Err(err))
		a.cache.Add(challengeHash, &challengeVerifierResult{hash: hash, err: err})
	}
	return hash, err
}

func NewCachingChallengeVerifier(size int, verifier types.ChallengeVerifier) (types.ChallengeVerifier, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &cachingChallengeVerifier{
		cache:    cache,
		verifier: verifier,
	}, nil
}
