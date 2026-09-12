package mediaasset

import (
	"encoding/json"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/ingestvalidation"
)

func mapIngestValidationError(err error) error {
	switch {
	case ingestvalidation.IsUnsupported(err):
		return ErrContentUnsupported
	case ingestvalidation.IsMalformed(err):
		return ErrContentMalformed
	case ingestvalidation.IsMismatch(err):
		return ErrContentMismatch
	case ingestvalidation.IsTimeout(err):
		return ErrContentValidationTimeout
	case ingestvalidation.IsInfrastructure(err):
		return ErrContentValidationFailed
	case ingestvalidation.IsTooLarge(err):
		return ErrTooLarge
	default:
		return ErrContentValidationFailed
	}
}

func toIngestKind(kind Kind) ingestvalidation.Kind {
	switch kind {
	case KindImage:
		return ingestvalidation.KindImage
	case KindVideo:
		return ingestvalidation.KindVideo
	case KindAudio:
		return ingestvalidation.KindAudio
	case KindDocument:
		return ingestvalidation.KindDocument
	default:
		return ""
	}
}

func fromIngestKind(kind ingestvalidation.Kind) Kind {
	switch kind {
	case ingestvalidation.KindImage:
		return KindImage
	case ingestvalidation.KindVideo:
		return KindVideo
	case ingestvalidation.KindAudio:
		return KindAudio
	case ingestvalidation.KindDocument:
		return KindDocument
	default:
		return KindOther
	}
}

func mergeDeclaredMIME(metadata json.RawMessage, declaredMIME, verifiedMIME string) json.RawMessage {
	declaredMIME = strings.ToLower(strings.TrimSpace(declaredMIME))
	verifiedMIME = strings.ToLower(strings.TrimSpace(verifiedMIME))
	if declaredMIME == "" || declaredMIME == verifiedMIME {
		return metadata
	}
	var payload map[string]any
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["declared_mime_type"] = declaredMIME
	encoded, err := json.Marshal(payload)
	if err != nil {
		return metadata
	}
	return encoded
}
