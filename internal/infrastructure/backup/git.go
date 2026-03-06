package backup

import (
	"context"
	"errors"
	"fmt"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	gitAuthorName  = "ingatan backup"
	gitAuthorEmail = "backup@ingatan"
	gitRemoteName  = "origin"
)

// GitConfig holds git backup configuration.
type GitConfig struct {
	// RemoteURL is the optional remote repository URL.
	// If empty, only a local commit is created (no push).
	RemoteURL string
	// Branch is the branch name to commit to. Defaults to "main".
	Branch string
}

// GitBackup backs up the data directory by committing it to a local git repository
// and optionally pushing to a remote.
// Implements Backuper.
type GitBackup struct {
	cfg GitConfig
}

// NewGitBackup creates a new GitBackup with the provided configuration.
func NewGitBackup(cfg GitConfig) *GitBackup {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	return &GitBackup{cfg: cfg}
}

// Backup initializes or opens a git repository at dataDir, stages all files,
// commits with a timestamp message, and optionally pushes to the configured remote.
func (b *GitBackup) Backup(_ context.Context, dataDir string) error {
	repo, err := openOrInitRepo(dataDir)
	if err != nil {
		return fmt.Errorf("git backup: open/init repo: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git backup: get worktree: %w", err)
	}

	if err := w.AddGlob("."); err != nil {
		return fmt.Errorf("git backup: stage files: %w", err)
	}

	msg := "backup: " + time.Now().UTC().Format(time.RFC3339)
	sig := &object.Signature{
		Name:  gitAuthorName,
		Email: gitAuthorEmail,
		When:  time.Now(),
	}

	_, err = w.Commit(msg, &gogit.CommitOptions{
		Author:    sig,
		Committer: sig,
	})
	if err != nil {
		return fmt.Errorf("git backup: commit: %w", err)
	}

	if b.cfg.RemoteURL != "" {
		if err := b.pushToRemote(repo); err != nil {
			return fmt.Errorf("git backup: push: %w", err)
		}
	}

	return nil
}

// Name returns "git".
func (b *GitBackup) Name() string { return "git" }

// openOrInitRepo opens an existing git repository at path, or initializes a new one.
func openOrInitRepo(path string) (*gogit.Repository, error) {
	repo, err := gogit.PlainOpen(path)
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, gogit.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	repo, err = gogit.PlainInit(path, false)
	if err != nil {
		return nil, fmt.Errorf("init repo: %w", err)
	}
	return repo, nil
}

// pushToRemote ensures the "origin" remote is set to RemoteURL and pushes.
func (b *GitBackup) pushToRemote(repo *gogit.Repository) error {
	_, err := repo.Remote(gitRemoteName)
	if errors.Is(err, gogit.ErrRemoteNotFound) {
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: gitRemoteName,
			URLs: []string{b.cfg.RemoteURL},
		})
		if err != nil {
			return fmt.Errorf("create remote: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("get remote: %w", err)
	}

	err = repo.Push(&gogit.PushOptions{
		RemoteName: gitRemoteName,
	})
	if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}
