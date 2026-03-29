package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gca/internal/ai"
	"gca/internal/editor"
	"gca/internal/git"
	"gca/internal/prompt"

	"github.com/spf13/cobra"
)

type splitFlags struct {
	dryRun   bool
	jsonOut  bool
	provider string
	model    string
}

func newSplitCmd() *cobra.Command {
	f := &splitFlags{}

	cmd := &cobra.Command{
		Use:   "split",
		Short: "Split staged changes into logical commits",
		Long:  "Analyze staged changes, group them logically, and generate AI-powered commit messages for each group.",
		Example: `  gca split                    Split and commit interactively
  gca split -n                 Preview the split plan
  gca split --json             Get split plan as JSON
  gca split -p cerebras        Use Cerebras for grouping`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSplit(cmd.Context(), f)
		},
	}

	cmd.Flags().BoolVarP(&f.dryRun, "dry-run", "n", false, "Preview split plan without committing")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Output split plan as JSON")
	cmd.Flags().StringVarP(&f.provider, "provider", "p", "", "AI provider (groq, cerebras)")
	cmd.Flags().StringVarP(&f.model, "model", "m", "", "Override AI model")

	return cmd
}

func runSplit(ctx context.Context, f *splitFlags) error {
	out := NewFormatter(os.Stdout, os.Stderr)

	// Check we're in a git repo
	inside, err := git.IsInsideWorkTree()
	if err != nil || !inside {
		return &ExitError{Err: fmt.Errorf("not a git repository"), Code: 2}
	}

	// Resolve config
	cfg, err := resolveConfig(f.provider, f.model)
	if err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	// Check for staged changes
	hasChanges, err := git.HasStagedChanges()
	if err != nil {
		return &ExitError{Err: fmt.Errorf("checking staged changes: %v", err), Code: 2}
	}
	if !hasChanges {
		return &ExitError{
			Err:  fmt.Errorf("no staged changes. Stage files with 'git add' first"),
			Code: 1,
		}
	}

	// Get diff and file list
	diff, err := git.StagedDiff()
	if err != nil {
		return &ExitError{Err: err, Code: 2}
	}

	files, err := git.StagedFiles()
	if err != nil {
		return &ExitError{Err: err, Code: 2}
	}

	// Create AI provider
	provider, err := ai.NewProvider(ai.ProviderConfig{
		Name:   cfg.Provider,
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	})
	if err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	// Group changes via AI
	out.Status("Analyzing staged changes...")
	groups, err := provider.GroupChanges(ctx, diff, files)
	if err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	// Validate: filter unknown files, assign orphans
	groups = validateGroups(groups, files)

	// JSON output
	if f.jsonOut {
		return outputJSON(out, groups)
	}

	// Display the plan
	out.Status(fmt.Sprintf("Splitting into %d commits:\n", len(groups)))
	displayGroups(out, groups)

	// Dry run — stop here
	if f.dryRun {
		fmt.Fprintln(out.Stderr)
		out.Status("(dry run — no commits created)")
		return nil
	}

	// Interactive review
	choice, err := prompt.ConfirmWithPrompt(os.Stderr, os.Stdin, "\nApply all? (y/n/e): ")
	if err != nil {
		return &ExitError{Err: fmt.Errorf("reading input: %v", err), Code: 1}
	}

	switch choice {
	case prompt.Accept:
		return applyGroups(out, groups)
	case prompt.Edit:
		return applyGroupsInteractive(out, groups)
	case prompt.Reject:
		out.Status("Aborted.")
		return nil
	}

	return nil
}

func validateGroups(groups []ai.CommitGroup, stagedFiles []string) []ai.CommitGroup {
	staged := make(map[string]bool, len(stagedFiles))
	for _, f := range stagedFiles {
		staged[f] = true
	}

	seen := make(map[string]bool)
	for i := range groups {
		var valid []string
		for _, f := range groups[i].Files {
			if staged[f] && !seen[f] {
				valid = append(valid, f)
				seen[f] = true
			}
		}
		groups[i].Files = valid
	}

	// Remove empty groups
	var result []ai.CommitGroup
	for _, g := range groups {
		if len(g.Files) > 0 {
			result = append(result, g)
		}
	}

	// Assign orphaned files to the last group
	var orphans []string
	for _, f := range stagedFiles {
		if !seen[f] {
			orphans = append(orphans, f)
		}
	}
	if len(orphans) > 0 && len(result) > 0 {
		result[len(result)-1].Files = append(result[len(result)-1].Files, orphans...)
	}

	return result
}

func displayGroups(out *Formatter, groups []ai.CommitGroup) {
	for i, g := range groups {
		out.CommitGroupHeader(i+1, len(groups), g.Title, g.Body, g.Files)
	}
}

func outputJSON(out *Formatter, groups []ai.CommitGroup) error {
	data := struct {
		Commits []ai.CommitGroup `json:"commits"`
	}{Commits: groups}

	enc := json.NewEncoder(out.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func applyGroups(out *Formatter, groups []ai.CommitGroup) error {
	if err := git.ResetStaged(); err != nil {
		return &ExitError{Err: fmt.Errorf("unstaging files: %v", err), Code: 2}
	}

	for i, g := range groups {
		if err := git.StageFiles(g.Files); err != nil {
			return &ExitError{
				Err:  fmt.Errorf("staging files for commit %d/%d: %v", i+1, len(groups), err),
				Code: 2,
			}
		}

		message := g.Title + "\n\n" + g.Body
		hash, err := git.Commit(message)
		if err != nil {
			return &ExitError{
				Err:  fmt.Errorf("commit %d/%d failed: %v", i+1, len(groups), err),
				Code: 2,
			}
		}
		out.Success(fmt.Sprintf("[%s] %s", hash, g.Title))
	}
	return nil
}

func applyGroupsInteractive(out *Formatter, groups []ai.CommitGroup) error {
	if err := git.ResetStaged(); err != nil {
		return &ExitError{Err: fmt.Errorf("unstaging files: %v", err), Code: 2}
	}

	for i, g := range groups {
		out.CommitGroupHeader(i+1, len(groups), g.Title, g.Body, g.Files)

		choice, err := prompt.ConfirmWithPrompt(os.Stderr, os.Stdin,
			fmt.Sprintf("  Apply commit %d/%d? (y/n/e): ", i+1, len(groups)))
		if err != nil {
			return &ExitError{Err: fmt.Errorf("reading input: %v", err), Code: 1}
		}

		message := g.Title + "\n\n" + g.Body

		switch choice {
		case prompt.Accept:
			// apply as-is
		case prompt.Edit:
			edited, err := editor.Edit(message)
			if err != nil {
				return &ExitError{Err: err, Code: 1}
			}
			message = edited
		case prompt.Reject:
			out.Status(fmt.Sprintf("  Skipped commit %d/%d", i+1, len(groups)))
			continue
		}

		if err := git.StageFiles(g.Files); err != nil {
			return &ExitError{
				Err:  fmt.Errorf("staging files for commit %d/%d: %v", i+1, len(groups), err),
				Code: 2,
			}
		}

		hash, err := git.Commit(message)
		if err != nil {
			return &ExitError{
				Err:  fmt.Errorf("commit %d/%d failed: %v", i+1, len(groups), err),
				Code: 2,
			}
		}
		out.Success(fmt.Sprintf("[%s] %s", hash, g.Title))
	}
	return nil
}
