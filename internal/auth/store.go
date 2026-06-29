// Package auth owns the scoped-token store used to gate end-user access to
// the LetoRollout console. Tokens are persisted as a single JSON file so the
// service stays a zero-dependency single binary.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors returned by TokenStore operations.
var (
	ErrUnauthorized = errors.New("token missing or invalid")
	ErrTokenExpired = errors.New("token expired")
	ErrNotFound     = errors.New("token not found")
)

// TokenScope is one authorized range. A namespace-only scope authorizes every
// Deployment in that namespace; a scope with Deployment set authorizes only
// that single Deployment.
type TokenScope struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment,omitempty"`
}

// TokenRecord is one stored token.
type TokenRecord struct {
	ID        string       `json:"id"`                 // UUID; the API deletes by id
	Token     string      `json:"token"`              // bearer value; random 32-byte hex when generated
	Label     string      `json:"label"`              // optional human note, e.g. "alice-prod"
	Scopes    []TokenScope `json:"scopes"`
	ExpiresAt *time.Time   `json:"expiresAt,omitempty"` // nil = never expires
	CreatedAt time.Time    `json:"createdAt"`
}

type fileFormat struct {
	Tokens []TokenRecord `json:"tokens"`
}

// TokenStore is an in-memory mirror of the JSON token file, guarded by a
// mutex. LoadStore reads the file once; every mutating call writes back
// atomically.
type TokenStore struct {
	mu     sync.RWMutex
	path   string
	tokens []TokenRecord
}

// LoadStore reads the token file at path. A missing file yields an empty
// store rather than an error, so first run with no tokens is valid.
func LoadStore(path string) (*TokenStore, error) {
	s := &TokenStore{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.tokens = f.Tokens
	return s, nil
}

// Create adds a token record, filling id/token/createdAt when unset, and
// writes back to disk. The created record (with plaintext token) is returned
// once so the admin can hand it to the user.
func (s *TokenStore) Create(r TokenRecord) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.Token == "" {
		r.Token = randomToken()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.tokens = append(s.tokens, r)
	if err := s.writeLocked(); err != nil {
		// roll back the in-memory append so state stays consistent
		s.tokens = s.tokens[:len(s.tokens)-1]
		return TokenRecord{}, err
	}
	return r, nil
}

// List returns all records with the plaintext Token field cleared. The admin
// UI deletes by id, so the token value is never exposed after creation.
func (s *TokenStore) List() []TokenRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TokenRecord, len(s.tokens))
	for i, t := range s.tokens {
		t.Token = "" // never expose plaintext
		out[i] = t
	}
	return out
}

// Delete removes the record with the given id and writes back.
func (s *TokenStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tokens {
		if t.ID == id {
			s.tokens = append(s.tokens[:i], s.tokens[i+1:]...)
			return s.writeLocked()
		}
	}
	return ErrNotFound
}

// Verify looks up a token by its bearer value using a constant-time compare
// and rejects expired tokens.
func (s *TokenStore) Verify(token string) (TokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tokens {
		if subtle.ConstantTimeCompare([]byte(t.Token), []byte(token)) == 1 {
			if t.ExpiresAt != nil && time.Now().UTC().After(*t.ExpiresAt) {
				return TokenRecord{}, ErrTokenExpired
			}
			return t, nil
		}
	}
	return TokenRecord{}, ErrUnauthorized
}

// Allows reports whether this record authorizes the given namespace/deployment.
func (r TokenRecord) Allows(namespace, deployment string) bool {
	for _, sc := range r.Scopes {
		if sc.Namespace != namespace {
			continue
		}
		if sc.Deployment == "" || sc.Deployment == deployment {
			return true
		}
	}
	return false
}

func (s *TokenStore) writeLocked() error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(fileFormat{Tokens: s.tokens}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
