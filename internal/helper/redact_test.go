package helper

import (
	"strings"
	"testing"

	"cpa-usage-keeper/internal/entities"
)

func TestRedactSensitiveValueUsesCanonicalFormat(t *testing.T) {
	if got := RedactSensitiveValue("sk-BabcdefghijklmnopqrstuvwxyzmaWyTA"); got != "sk-*********maWyTA" {
		t.Fatalf("expected canonical masked key, got %q", got)
	}
	if got := RedactSensitiveValue("short"); got != "*********" {
		t.Fatalf("expected short key to use fixed mask, got %q", got)
	}
	if got := RedactSensitiveValue("sk-123456"); got != "*********" {
		t.Fatalf("expected boundary-length key to be fully masked, got %q", got)
	}
	if got := RedactSensitiveValue(""); got != "unknown" {
		t.Fatalf("expected empty key to stay compatible with public fallback, got %q", got)
	}
	if got := RedactSensitiveValue("unknown"); got != "unknown" {
		t.Fatalf("expected unknown key to remain unknown, got %q", got)
	}
}

func TestSensitiveValueAliasReturnsStableNonDisplayAlias(t *testing.T) {
	first := SensitiveValueAlias("sk-first-openai-key")
	second := SensitiveValueAlias("sk-second-openai-key")

	if first == second {
		t.Fatalf("expected different source aliases for different keys, got %q", first)
	}
	if !strings.HasPrefix(first, sensitiveValueAliasPrefix) || strings.Contains(first, "sk-") {
		t.Fatalf("expected non-sensitive alias, got %q", first)
	}
	if got := SensitiveValueAlias(first); got != first {
		t.Fatalf("expected existing alias to remain stable, got %q", got)
	}
}

func TestCPAAPIKeyDisplayNamePrefersAlias(t *testing.T) {
	row := entities.CPAAPIKey{APIKey: "sk-alpha123456", KeyAlias: "  Production  ", DisplayKey: "sk-B********************************Zejy"}

	if got := CPAAPIKeyDisplayName(row); got != "Production" {
		t.Fatalf("expected alias label, got %q", got)
	}
}

func TestCPAAPIKeyDisplayNameFallsBackToMaskedRawKey(t *testing.T) {
	row := entities.CPAAPIKey{APIKey: "sk-alpha123456", DisplayKey: "sk-B********************************Zejy"}

	if got := CPAAPIKeyDisplayName(row); got != "sk-*********123456" {
		t.Fatalf("expected canonical masked key fallback, got %q", got)
	}
}

func TestCPAAPIKeyMaskedDisplayKeyMasksRawKeyWithCanonicalFormat(t *testing.T) {
	row := entities.CPAAPIKey{APIKey: "sk-BabcdefghijklmnopqrstuvwxyzmaWyTA", DisplayKey: "sk-B********************************maWy"}

	if got := CPAAPIKeyMaskedDisplayKey(row); got != "sk-*********maWyTA" {
		t.Fatalf("expected canonical display key, got %q", got)
	}
}

func TestCPAAPIKeyMaskedDisplayKeyFallsBackToStoredDisplayKeyWhenRawKeyIsMissing(t *testing.T) {
	row := entities.CPAAPIKey{DisplayKey: "sk-*********maWyTA"}

	if got := CPAAPIKeyMaskedDisplayKey(row); got != "sk-*********maWyTA" {
		t.Fatalf("expected stored display key fallback, got %q", got)
	}
}
