package stockmedia

import "context"

type Provider interface {
	Search(context.Context, SearchRequest) (SearchPage, error)
	OpenForAcquisition(context.Context, string, MediaKind) (RemoteAsset, error)
}

type RemoteAsset interface {
	ContentType() string
	Open(context.Context) (ReadCloser, error)
}

type ReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}
