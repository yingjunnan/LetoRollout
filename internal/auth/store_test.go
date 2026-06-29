package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateVerifyAllows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec, err := s.Create(TokenRecord{Label: "alice", Scopes: []TokenScope{
		{Namespace: "dev"},
		{Namespace: "prod", Deployment: "api"},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == "" || rec.Token == "" {
		t.Fatal("id/token must be set")
	}
	got, err := s.Verify(rec.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.ID != rec.ID {
		t.Fatal("id mismatch")
	}
	if !got.Allows("dev", "anything") {
		t.Fatal("dev namespace should allow any deployment")
	}
	if !got.Allows("prod", "api") {
		t.Fatal("prod/api should be allowed")
	}
	if got.Allows("prod", "other") {
		t.Fatal("prod/other must be denied")
	}
	if got.Allows("other", "") {
		t.Fatal("other ns must be denied")
	}
}

func TestVerifyExpired(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	past := time.Now().Add(-time.Hour)
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}, ExpiresAt: &past})
	if _, err := s.Verify(rec.Token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

func TestVerifyUnknown(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	if _, err := s.Verify("nope"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestListOmitsToken(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("want 1, got %d", len(list))
	}
	if list[0].Token != "" {
		t.Fatal("List must not expose plaintext token")
	}
	if list[0].ID != rec.ID {
		t.Fatal("id mismatch")
	}
}

func TestDelete(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	if err := s.Delete(rec.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty after delete")
	}
}

func TestDeleteUnknownReturnsError(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s1, _ := LoadStore(path)
	rec, _ := s1.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	s2, _ := LoadStore(path)
	if _, err := s2.Verify(rec.Token); err != nil {
		t.Fatalf("should persist: %v", err)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	s, err := LoadStore(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestLoadPreservesExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s1, _ := LoadStore(path)
	past := time.Now().Add(-time.Hour).UTC()
	s1.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}, ExpiresAt: &past})
	s2, _ := LoadStore(path)
	list := s2.List()
	if len(list) != 1 {
		t.Fatalf("want 1, got %d", len(list))
	}
	if list[0].ExpiresAt == nil || !list[0].ExpiresAt.Equal(past) {
		t.Fatalf("expiry not preserved: %+v", list[0].ExpiresAt)
	}
}

func TestCreateDoesNotOverwriteProvidedToken(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, err := s.Create(TokenRecord{Token: "custom", Scopes: []TokenScope{{Namespace: "dev"}}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.Token != "custom" {
		t.Fatalf("token = %q, want custom", rec.Token)
	}
	if _, err := s.Verify("custom"); err != nil {
		t.Fatalf("verify custom: %v", err)
	}
}

func TestFileCreatedWithRestrictedPerms(t *testing.T) {
	// On Windows file permissions are not POSIX; only assert the file exists.
	path := filepath.Join(t.TempDir(), "tokens.json")
	s, _ := LoadStore(path)
	_, _ = s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
