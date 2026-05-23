package jwtstore

import (
	"testing"

	"github.com/gomooth/pkg/http/jwt"
)

func TestDefaultHashFunc(t *testing.T) {
	token := "test-token-123"
	hashed := jwt.DefaultHashFunc(token)

	if hashed == token {
		t.Error("DefaultHashFunc should not return the original token")
	}
	if len(hashed) != 64 {
		t.Errorf("SHA256 hex should be 64 chars, got %d", len(hashed))
	}

	hashed2 := jwt.DefaultHashFunc(token)
	if hashed != hashed2 {
		t.Error("DefaultHashFunc should be deterministic")
	}

	hashed3 := jwt.DefaultHashFunc("different-token")
	if hashed == hashed3 {
		t.Error("DefaultHashFunc should produce different hashes for different inputs")
	}
}

func TestIdentityHash(t *testing.T) {
	token := "test-token-123"
	result := jwt.IdentityHash(token)

	if result != token {
		t.Errorf("IdentityHash should return the original token, got %s", result)
	}
}

func TestNewSingleRedisStore_DefaultHash(t *testing.T) {
	store := NewSingleRedisStore(nil).(*singleRedisStore)
	if store.hashFunc == nil {
		t.Error("hashFunc should not be nil")
	}
	token := "test"
	if store.hashFunc(token) == token {
		t.Error("default hashFunc should hash, not identity")
	}
}

func TestNewSingleRedisStore_WithIdentityHash(t *testing.T) {
	store := NewSingleRedisStore(nil, WithSingleHashFunc(jwt.IdentityHash)).(*singleRedisStore)
	token := "test"
	if store.hashFunc(token) != token {
		t.Error("identity hash should return original token")
	}
}

func TestNewMultiRedisStore_DefaultHash(t *testing.T) {
	store := NewMultiRedisStore(nil).(*multiRedisStore)
	if store.hashFunc == nil {
		t.Error("hashFunc should not be nil")
	}
	token := "test"
	if store.hashFunc(token) == token {
		t.Error("default hashFunc should hash, not identity")
	}
}

func TestNewMultiRedisStore_WithIdentityHash(t *testing.T) {
	store := NewMultiRedisStore(nil, WithMultiHashFunc(jwt.IdentityHash)).(*multiRedisStore)
	token := "test"
	if store.hashFunc(token) != token {
		t.Error("identity hash should return original token")
	}
}
