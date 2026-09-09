package ingestvalidation

import "errors"

var (
	ErrUnsupported    = errors.New("ingest: content type is unsupported")
	ErrMalformed      = errors.New("ingest: content is malformed")
	ErrMismatch       = errors.New("ingest: content does not match declared type")
	ErrTimeout        = errors.New("ingest: content validation timed out")
	ErrInfrastructure = errors.New("ingest: content validation infrastructure failed")
	ErrTooLarge       = errors.New("ingest: content exceeds limit")
)
