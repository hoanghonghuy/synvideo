package providers

import "strings"

// ProviderID is a stable provider identity separate from model identity.
type ProviderID string

// ModelID is a stable model identity scoped to a provider.
type ModelID string

func (id ProviderID) String() string {
	return string(id)
}

func (id ProviderID) Valid() bool {
	return isStableIdentifier(string(id))
}

func (id ModelID) String() string {
	return string(id)
}

func (id ModelID) Valid() bool {
	return isStableIdentifier(string(id))
}

func isStableIdentifier(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed != value {
		return false
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
