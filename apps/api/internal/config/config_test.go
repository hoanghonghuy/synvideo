package config

import "testing"

func TestConfigValidateAcceptsDefaults(t *testing.T) {
	cfg := Config{Addr: ":8080", Environment: "development"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to validate: %v", err)
	}
}

func TestConfigValidateRejectsInvalidEnvironment(t *testing.T) {
	cfg := Config{Addr: ":8080", Environment: "staging"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid environment to fail validation")
	}
}

func TestConfigValidateRejectsInvalidAddress(t *testing.T) {
	cfg := Config{Addr: "8080", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid address to fail validation")
	}
}

func TestConfigValidateRejectsNonNumericPort(t *testing.T) {
	cfg := Config{Addr: ":abc", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-numeric port to fail validation")
	}
}

func TestConfigValidateRejectsOutOfRangePort(t *testing.T) {
	cfg := Config{Addr: ":70000", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected out-of-range port to fail validation")
	}
}

func TestConfigValidateDoesNotResolveHostnames(t *testing.T) {
	cfg := Config{Addr: "api.internal.invalid:8080", Environment: "development"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected hostname syntax to validate without DNS lookup: %v", err)
	}
}
