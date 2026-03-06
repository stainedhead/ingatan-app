// Package backup provides backup implementations for ingatan data.
package backup

import "context"

// Backuper can back up the ingatan data directory.
type Backuper interface {
	// Backup backs up all data from dataDir.
	// Returns nil on success.
	Backup(ctx context.Context, dataDir string) error

	// Name returns the backup provider name (e.g., "s3", "git").
	Name() string
}
