package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/packages"
)

const (
	RepoRetrieverHeadErrMsg = "repository head error"
	RepoRetrieverPullErrMsg = "repository pull error"
	RepoRetrieverLoadErrMsg = "repository load error"
)

var (
	ErrRepoRetrieverHead = errors.New(RepoRetrieverHeadErrMsg)
	ErrRepoRetrieverPull = errors.New(RepoRetrieverPullErrMsg)
	ErrRepoRetrieverLoad = errors.New(RepoRetrieverLoadErrMsg)
)

type RepoRetriever interface {
	Digest(ref string) (string, error)
	Retrieve(ctx context.Context, image string) (*packages.RepositoryIndex, error)
}

type CraneRepoRetriever struct{}

func (r *CraneRepoRetriever) Digest(ref string) (string, error) {
	dsc, err := crane.Head(ref)
	if err != nil {
		return "", fmt.Errorf("%s: %w", RepoRetrieverHeadErrMsg, err)
	}
	return fmt.Sprintf("%s:%s", dsc.Digest.Algorithm, dsc.Digest.Hex), nil
}

func (r *CraneRepoRetriever) Retrieve(ctx context.Context, ref string) (*packages.RepositoryIndex, error) {
	image, err := crane.Pull(ref)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RepoRetrieverPullErrMsg, err)
	}
	idx, err := packages.LoadRepositoryFromOCI(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RepoRetrieverLoadErrMsg, err)
	}
	return idx, nil
}
