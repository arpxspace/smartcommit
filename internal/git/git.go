package git

import (
	"fmt"
	"os/exec"
)

// IsRepo checks if the current directory is a git repository.
func IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// GetStagedDiff returns the diff of staged changes.
func GetStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff: %w", err)
	}
	return string(out), nil
}

// GetRecentHistory returns the last n commit messages with their bodies.
func GetRecentHistory(n int) (string, error) {
	// Format: Hash | Subject | Body
	// We use a custom format to make parsing easier if needed, but for AI context, raw text is often fine.
	// %h: abbreviated commit hash
	// %s: subject
	// %b: body
	format := "Commit: %h\nSubject: %s\nBody:\n%b\n---"
	cmd := exec.Command("git", "log", fmt.Sprintf("-n%d", n), fmt.Sprintf("--pretty=format:%s", format))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git history: %w", err)
	}
	return string(out), nil
}

// CommitCmd returns the exec.Cmd for the git commit command with the given message.
// It uses the -e flag to open the editor.
// If message is empty, it runs 'git commit' without -m, opening the editor for a manual commit.
func CommitCmd(message string) *exec.Cmd {
	if message == "" {
		return exec.Command("git", "commit")
	}
	return exec.Command("git", "commit", "-e", "-m", message)
}

// GetStagedDiffSize returns the approximate number of characters in the staged diff.
// This is used to warn the user if the diff is too large for the AI context.
func GetStagedDiffSize() (int, error) {
	diff, err := GetStagedDiff()
	if err != nil {
		return 0, err
	}
	return len(diff), nil
}
