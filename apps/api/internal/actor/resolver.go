package actor

import (
	"errors"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

var ErrNoPrincipal = errors.New("request principal unavailable")

type Resolver interface {
	Resolve(r *http.Request) (project.Principal, error)
}

type LocalResolver struct {
	cfg config.Config
}

func NewLocalResolver(cfg config.Config) *LocalResolver {
	return &LocalResolver{cfg: cfg}
}

func (r *LocalResolver) Resolve(_ *http.Request) (project.Principal, error) {
	if r.cfg.Environment == config.EnvironmentProduction {
		return project.Principal{}, ErrNoPrincipal
	}
	if r.cfg.LocalActorID == nil {
		return project.Principal{}, ErrNoPrincipal
	}
	return project.Principal{OwnerID: *r.cfg.LocalActorID}, nil
}
