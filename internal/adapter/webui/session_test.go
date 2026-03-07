package webui

import (
	"sync"
	"testing"
	"time"
)

func TestSessionStore_CreateAndValid(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	id := store.Create()
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}
	if !store.Valid(id) {
		t.Fatal("expected session to be valid immediately after creation")
	}
}

func TestSessionStore_Invalid_Unknown(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	if store.Valid("nonexistent-id") {
		t.Fatal("expected unknown session ID to be invalid")
	}
}

func TestSessionStore_Expired(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	defer store.Close()

	id := store.Create()
	time.Sleep(20 * time.Millisecond)
	if store.Valid(id) {
		t.Fatal("expected expired session to be invalid")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	id := store.Create()
	store.Delete(id)
	if store.Valid(id) {
		t.Fatal("expected deleted session to be invalid")
	}
}

func TestSessionStore_Delete_Idempotent(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	// Deleting a non-existent session must not panic.
	store.Delete("nonexistent-id")
}

func TestSessionStore_Concurrent(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	const n = 100
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i] = store.Create()
		}(i)
	}
	wg.Wait()

	for _, id := range ids {
		if !store.Valid(id) {
			t.Errorf("expected concurrently created session %s to be valid", id)
		}
	}
}

func TestSessionStore_UniqueIDs(t *testing.T) {
	store := NewSessionStore(time.Hour)
	defer store.Close()

	seen := make(map[string]bool, 100)
	for range 100 {
		id := store.Create()
		if seen[id] {
			t.Fatalf("duplicate session ID generated: %s", id)
		}
		seen[id] = true
	}
}
