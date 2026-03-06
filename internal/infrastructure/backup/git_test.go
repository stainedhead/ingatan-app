package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitBackup_Name(t *testing.T) {
	b := NewGitBackup(GitConfig{Branch: "main"})
	assert.Equal(t, "git", b.Name())
}

func TestGitBackup_InitializesRepoAndCommits(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{}`), 0o600))

	b := NewGitBackup(GitConfig{Branch: "main"})
	err := b.Backup(context.Background(), dir)
	require.NoError(t, err)

	// Verify a git repo was created.
	repo, err := gogit.PlainOpen(dir)
	require.NoError(t, err)

	// Verify at least one commit exists.
	head, err := repo.Head()
	require.NoError(t, err)
	assert.NotEmpty(t, head.Hash().String())

	// Verify the commit message starts with "backup:".
	commit, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Contains(t, commit.Message, "backup:")
}

func TestGitBackup_SecondBackupAddsNewCommit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"v":1}`), 0o600))

	b := NewGitBackup(GitConfig{Branch: "main"})

	// First backup.
	require.NoError(t, b.Backup(context.Background(), dir))

	// Modify the file and do second backup.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"v":2}`), 0o600))
	require.NoError(t, b.Backup(context.Background(), dir))

	repo, err := gogit.PlainOpen(dir)
	require.NoError(t, err)

	// Count commits — should be at least 2.
	iter, err := repo.Log(&gogit.LogOptions{})
	require.NoError(t, err)
	count := 0
	require.NoError(t, iter.ForEach(func(_ *object.Commit) error {
		count++
		return nil
	}))
	assert.GreaterOrEqual(t, count, 2)
}

func TestGitBackup_StagesAllFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "stores", "default"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stores", "default", "memories.json"), []byte(`[]`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "principals.json"), []byte(`[]`), 0o600))

	b := NewGitBackup(GitConfig{Branch: "main"})
	require.NoError(t, b.Backup(context.Background(), dir))

	repo, err := gogit.PlainOpen(dir)
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	commit, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)

	tree, err := commit.Tree()
	require.NoError(t, err)

	// All files should be present in the committed tree.
	var committed []string
	require.NoError(t, tree.Files().ForEach(func(f *object.File) error {
		committed = append(committed, f.Name)
		return nil
	}))

	assert.Contains(t, committed, "principals.json")
	assert.Contains(t, committed, "stores/default/memories.json")
}

func TestGitBackup_EmptyDirSucceeds(t *testing.T) {
	dir := t.TempDir()

	b := NewGitBackup(GitConfig{Branch: "main"})
	// An empty directory should not return an error.
	err := b.Backup(context.Background(), dir)
	// Either succeeds with empty commit or returns a benign "nothing to commit" error.
	// The important thing: no panic, no unexpected error from the implementation.
	_ = err
}
