package stockmedia

import "context"

type Provider interface {
	Search(context.Context, SearchRequest) (SearchPage, error)
	ResolveForAcquisition(context.Context, string, MediaKind) (AcquisitionSource, error)
}

type AcquisitionSource struct {
	Result   SearchResult
	Filename string
	Remote   RemoteAsset
}

type RemoteAsset interface {
	ContentType() string
	Open(context.Context) (ReadCloser, error)
}

type ReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}
