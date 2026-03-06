package domain

import "time"

// PrincipalType distinguishes human users from agent clients.
type PrincipalType string

const (
	// PrincipalTypeHuman represents a human user.
	PrincipalTypeHuman PrincipalType = "human"
	// PrincipalTypeAgent represents an AI agent client.
	PrincipalTypeAgent PrincipalType = "agent"
)

// InstanceRole is the instance-level authorization role for a principal.
type InstanceRole string

const (
	// InstanceRoleUser is the default role for authenticated principals.
	InstanceRoleUser InstanceRole = "user"
	// InstanceRoleAdmin grants full instance access including principal management.
	InstanceRoleAdmin InstanceRole = "admin"
)

// Principal represents an authenticated identity — either a human or an agent.
type Principal struct {
	ID         string
	Name       string
	Type       PrincipalType
	Email      string // Empty for agents.
	Role       InstanceRole
	APIKeyHash string // SHA-256 hex hash of the API key; empty = revoked / not set.
	CreatedAt  time.Time
	LastSeenAt time.Time
}
