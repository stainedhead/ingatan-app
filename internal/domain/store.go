package domain

import "time"

// StoreRole is the per-store authorization role for a store member.
type StoreRole string

const (
	// StoreRoleOwner grants full control: read, write, delete, manage members.
	StoreRoleOwner StoreRole = "owner"
	// StoreRoleWriter grants read and write access to memories.
	StoreRoleWriter StoreRole = "writer"
	// StoreRoleReader grants read-only access to memories and search.
	StoreRoleReader StoreRole = "reader"
)

// StoreMember pairs a principal with their role in a store.
type StoreMember struct {
	PrincipalID string
	Role        StoreRole
}

// Store is a named, isolated collection of memories with its own HNSW and BM25 indexes.
// The store name is a slug matching [a-z0-9-]+, globally unique, and immutable after creation.
type Store struct {
	Name           string
	Description    string
	OwnerID        string
	CreatedAt      time.Time
	Members        []StoreMember
	EmbeddingModel string // Recorded on first memory save; immutable thereafter.
	EmbeddingDims  int    // Recorded alongside EmbeddingModel.
}

// IsPersonal reports whether this store is a personal store (owned and named after its owner).
func (s *Store) IsPersonal() bool {
	return s.Name == s.OwnerID
}

// MemberRole returns the role of the given principal in this store.
// Returns an empty string if the principal is not a member.
func (s *Store) MemberRole(principalID string) StoreRole {
	for _, m := range s.Members {
		if m.PrincipalID == principalID {
			return m.Role
		}
	}
	return ""
}
