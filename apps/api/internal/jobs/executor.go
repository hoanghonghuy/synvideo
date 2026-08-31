package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Handler interface {
	Handle(ctx context.Context, job Job) (result json.RawMessage, err error)
}

type HandlerFunc func(ctx context.Context, job Job) (result json.RawMessage, err error)

func (f HandlerFunc) Handle(ctx context.Context, job Job) (result json.RawMessage, err error) {
	return f(ctx, job)
}

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(kind string, handler Handler) error {
	if kind == "" || handler == nil {
		return errors.Join(ErrInvalidInput, errors.New("kind and handler cannot be empty"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[kind]; exists {
		return errors.Join(ErrInvalidInput, fmt.Errorf("handler for kind %q already registered", kind))
	}
	r.handlers[kind] = handler
	return nil
}

func (r *Registry) Get(kind string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[kind]
	return h, ok
}

func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		kinds = append(kinds, k)
	}
	return kinds
}

type ExecutorConfig struct {
	LeaseDuration  time.Duration
	PollInterval   time.Duration
	DefaultBackoff time.Duration
}

type Executor struct {
	repo     Repository
	registry *Registry
	config   ExecutorConfig
}

func NewExecutor(repo Repository, registry *Registry, cfg ExecutorConfig) *Executor {
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 30 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.DefaultBackoff <= 0 {
		cfg.DefaultBackoff = 10 * time.Second
	}
	return &Executor{
		repo:     repo,
		registry: registry,
		config:   cfg,
	}
}

func (e *Executor) RunOnce(ctx context.Context) (bool, error) {
	kinds := e.registry.Kinds()
	if len(kinds) == 0 {
		return false, nil
	}

	job, err := e.repo.ClaimNext(ctx, ClaimOptions{
		Kinds:         kinds,
		LeaseDuration: e.config.LeaseDuration,
	})
	if err != nil {
		if errors.Is(err, ErrNoJobAvailable) {
			return false, nil
		}
		return false, err
	}

	if job.LeaseToken == nil {
		return false, errors.New("jobs: claimed job has nil lease token")
	}
	leaseToken := *job.LeaseToken

	handler, ok := e.registry.Get(job.Kind)
	if !ok {
		_, _ = e.repo.MarkTerminalFailure(context.Background(), job.ID, leaseToken, "ERR_UNKNOWN_JOB_KIND")
		return true, ErrUnknownJobKind
	}

	result, handleErr := handler.Handle(ctx, job)
	if handleErr == nil {
		_, err := e.repo.MarkSuccess(context.Background(), job.ID, leaseToken, result)
		return true, err
	}

	// Context was canceled; do not corrupt state or mark failed, leave lease to expire and be reclaimed
	if ctx.Err() != nil {
		return true, ctx.Err()
	}

	var retryErr *RetryableJobError
	var termErr *TerminalJobError

	if errors.As(handleErr, &retryErr) {
		if job.Attempt < job.MaxAttempts {
			backoff := e.config.DefaultBackoff
			if retryErr.RetryAfter != nil && *retryErr.RetryAfter > 0 {
				backoff = *retryErr.RetryAfter
			}
			nextAvail := time.Now().UTC().Add(backoff)
			code := retryErr.Code
			if code == "" {
				code = "ERR_RETRYABLE"
			}
			_, err := e.repo.MarkRetryableFailure(context.Background(), job.ID, leaseToken, code, nextAvail)
			return true, err
		}
		code := retryErr.Code
		if code == "" {
			code = "ERR_MAX_ATTEMPTS_EXCEEDED"
		}
		_, err := e.repo.MarkTerminalFailure(context.Background(), job.ID, leaseToken, code)
		return true, err
	}

	if errors.As(handleErr, &termErr) {
		code := termErr.Code
		if code == "" {
			code = "ERR_TERMINAL"
		}
		_, err := e.repo.MarkTerminalFailure(context.Background(), job.ID, leaseToken, code)
		return true, err
	}

	// Unclassified generic error
	code := "ERR_JOB_FAILED"
	if job.Attempt < job.MaxAttempts {
		nextAvail := time.Now().UTC().Add(e.config.DefaultBackoff)
		_, err := e.repo.MarkRetryableFailure(context.Background(), job.ID, leaseToken, code, nextAvail)
		return true, err
	}
	_, err = e.repo.MarkTerminalFailure(context.Background(), job.ID, leaseToken, code)
	return true, err
}

func (e *Executor) Start(ctx context.Context) error {
	ticker := time.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for {
				executed, err := e.RunOnce(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return err
					}
					// Log/continue on transient error
					break
				}
				if !executed {
					break
				}
			}
		}
	}
}
