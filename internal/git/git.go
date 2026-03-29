package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitError struct {
	Op     string
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git %s: %s", e.Op, strings.TrimSpace(e.Stderr))
	}
	return fmt.Sprintf("git %s: %v", e.Op, e.Err)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

func IsInsideWorkTree() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false, &GitError{Op: "rev-parse", Err: err}
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err == nil {
		return false, nil // exit 0 = no changes
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil // exit 1 = has changes
	}
	return false, &GitError{Op: "diff", Err: err}
}

func StagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--no-color")
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return "", &GitError{Op: "diff", Stderr: stderr, Err: err}
	}
	return string(out), nil
}

func Commit(message string) (string, error) {
	cmd := exec.Command("git", "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &GitError{Op: "commit", Stderr: string(out), Err: err}
	}

	hashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hashOut, err := hashCmd.Output()
	if err != nil {
		// Commit succeeded but hash retrieval failed — not critical
		return "unknown", nil
	}
	return strings.TrimSpace(string(hashOut)), nil
}
