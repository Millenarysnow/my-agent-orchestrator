package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestLocalBaseIncludesUnpushedCommitsWithoutNetwork(t *testing.T) {
	git := requireGit(t)
	tmp := t.TempDir()
	repo := setupOriginClone(t, git, tmp)
	runGit(t, git, repo, "commit", "--allow-empty", "-m", "local only")
	want := gitOutput(t, git, repo, "rev-parse", "HEAD")
	runGit(t, git, repo, "remote", "set-url", "origin", "https://example.invalid/offline.git")
	ws, err := New(Options{Binary: git, ManagedRoot: filepath.Join(tmp, "managed"), RepoResolver: StaticRepoResolver{"proj": repo}})
	if err != nil {
		t.Fatal(err)
	}
	originalRun := ws.run
	ws.run = func(ctx context.Context, binary string, args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "fetch" || arg == "ls-remote" || arg == "pull" {
				t.Fatalf("local task attempted network command: %v", args)
			}
		}
		return originalRun(ctx, binary, args...)
	}
	info, err := ws.Create(context.Background(), ports.WorkspaceConfig{ProjectID: "proj", SessionID: "local", Branch: "task/local", BaseBranch: "refs/heads/main", FreshBranch: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, git, info.Path, "rev-parse", "HEAD"); got != want || info.BaseSHA != want || info.BaseRef != "refs/heads/main" {
		t.Fatalf("task head=%s info=%+v, want local base %s", got, info, want)
	}
}

func TestLocalBaseDeletedDoesNotFallBackToRemote(t *testing.T) {
	git := requireGit(t)
	tmp := t.TempDir()
	repo := setupOriginClone(t, git, tmp)
	runGit(t, git, repo, "checkout", "-b", "another")
	runGit(t, git, repo, "branch", "-D", "main")
	ws, err := New(Options{Binary: git, ManagedRoot: filepath.Join(tmp, "managed"), RepoResolver: StaticRepoResolver{"proj": repo}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ws.Create(context.Background(), ports.WorkspaceConfig{ProjectID: "proj", SessionID: "local", Branch: "task/local", BaseBranch: "refs/heads/main", FreshBranch: true})
	if !errors.Is(err, ErrDefaultBranchUnresolved) {
		t.Fatalf("create with deleted local base = %v", err)
	}
}

func TestLocalBasePinsCommitBeforeWorktreeAdd(t *testing.T) {
	git := requireGit(t)
	tmp := t.TempDir()
	repo := setupOriginClone(t, git, tmp)
	want := gitOutput(t, git, repo, "rev-parse", "HEAD")
	runGit(t, git, repo, "commit", "--allow-empty", "-m", "later")
	later := gitOutput(t, git, repo, "rev-parse", "HEAD")
	runGit(t, git, repo, "reset", "--hard", want)
	ws, err := New(Options{Binary: git, ManagedRoot: filepath.Join(tmp, "managed"), RepoResolver: StaticRepoResolver{"proj": repo}})
	if err != nil {
		t.Fatal(err)
	}
	originalRun := ws.run
	ws.run = func(ctx context.Context, binary string, args ...string) ([]byte, error) {
		if len(args) > 3 && args[2] == "worktree" && args[3] == "add" {
			runGit(t, git, repo, "update-ref", "refs/heads/main", later)
			if args[len(args)-1] != want {
				t.Fatalf("worktree start point is not pinned: %v", args)
			}
		}
		return originalRun(ctx, binary, args...)
	}
	info, err := ws.Create(context.Background(), ports.WorkspaceConfig{ProjectID: "proj", SessionID: "pinned", Branch: "task/pinned", BaseBranch: "refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, git, info.Path, "rev-parse", "HEAD"); got != want || info.BaseSHA != want {
		t.Fatalf("head=%s creation=%s want=%s", got, info.BaseSHA, want)
	}
}

func TestLocalBaseAutoSupportsExistingRemotelessRepository(t *testing.T) {
	git := requireGit(t)
	tmp := t.TempDir()
	repo := setupOriginClone(t, git, tmp)
	runGit(t, git, repo, "remote", "remove", "origin")
	ws, err := New(Options{Binary: git, ManagedRoot: filepath.Join(tmp, "managed"), RepoResolver: StaticRepoResolver{"proj": repo}})
	if err != nil {
		t.Fatal(err)
	}
	info, err := ws.Create(context.Background(), ports.WorkspaceConfig{ProjectID: "proj", SessionID: "offline", Branch: "task/offline"})
	if err != nil {
		t.Fatal(err)
	}
	if info.BaseRef != "refs/heads/main" {
		t.Fatalf("base ref = %q", info.BaseRef)
	}
	if _, err := os.Stat(filepath.Join(info.Path, "README.md")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBaseRestoreDoesNotReplaceLostTaskWithProjectBase(t *testing.T) {
	git := requireGit(t)
	tmp := t.TempDir()
	repo := setupOriginClone(t, git, tmp)
	ws, err := New(Options{Binary: git, ManagedRoot: filepath.Join(tmp, "managed"), RepoResolver: StaticRepoResolver{"proj": repo}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ws.Restore(context.Background(), ports.WorkspaceConfig{ProjectID: "proj", SessionID: "lost", Branch: "task/lost", BaseRef: "refs/heads/main"})
	if !errors.Is(err, ErrBranchNotFetched) || !strings.Contains(err.Error(), "cannot be restored") {
		t.Fatalf("restore lost task = %v", err)
	}
}
