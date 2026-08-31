package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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

type handlerResult struct {
	result json.RawMessage
	err    error
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

	result, handleErr, lifecycleErr := e.runHandler(ctx, handler, job, leaseToken)
	if lifecycleErr != nil {
		return true, lifecycleErr
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
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
			code = ErrorCodeMaxAttemptsExceeded
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

func (e *Executor) runHandler(ctx context.Context, handler Handler, job Job, leaseToken uuid.UUID) (json.RawMessage, error, error) {
	handlerCtx, cancelHandler := context.WithCancel(ctx)
	defer cancelHandler()

	resultCh := make(chan handlerResult, 1)
	go func() {
		result, err := handler.Handle(handlerCtx, job)
		resultCh <- handlerResult{result: result, err: err}
	}()

	renewInterval := e.config.LeaseDuration / 2
	if renewInterval <= 0 {
		renewInterval = time.Nanosecond
	}
	renewTicker := time.NewTicker(renewInterval)
	defer renewTicker.Stop()

	for {
		select {
		case result := <-resultCh:
			return result.result, result.err, nil
		case <-ctx.Done():
			cancelHandler()
			<-resultCh
			return nil, ctx.Err(), nil
		case <-renewTicker.C:
			renewCtx, cancelRenew := context.WithTimeout(context.Background(), renewInterval)
			_, err := e.repo.RenewLease(renewCtx, job.ID, leaseToken, e.config.LeaseDuration)
			cancelRenew()
			if err != nil {
				cancelHandler()
				<-resultCh
				return nil, nil, err
			}
		}
	}
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
