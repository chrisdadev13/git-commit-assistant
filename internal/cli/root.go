package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gca/internal/ai"
	"gca/internal/config"
	"gca/internal/editor"
	"gca/internal/git"
	"gca/internal/prompt"

	"github.com/spf13/cobra"
)

type ExitError struct {
	Err  error
	Code int
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

type flags struct {
	dryRun   bool
	noInput  bool
	json     bool
	provider string
	model    string
}

func NewRootCommand() *cobra.Command {
	f := &flags{}

	cmd := &cobra.Command{
		Use:   "gca",
		Short: "Git Commit Assistant — AI-powered commit messages",
		Long:  "Generate high-quality commit messages from your staged changes using fast AI inference.",
		Example: `  gca                          Generate and commit with AI message
  gca -n                       Preview message without committing
  gca --no-input               Auto-commit (CI/scripts)
  gca --json --dry-run         Get message as JSON
  gca -p cerebras              Use Cerebras instead of default

  gca config set provider groq Set default provider
  gca config set api-key KEY   Save your API key
  gca config list              Show current config`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), f)
		},
	}

	cmd.Flags().BoolVarP(&f.dryRun, "dry-run", "n", false, "Preview commit message without committing")
	cmd.Flags().BoolVar(&f.noInput, "no-input", false, "Auto-commit without prompting (CI/scripts)")
	cmd.Flags().BoolVar(&f.json, "json", false, "Output commit message as JSON")
	cmd.Flags().StringVarP(&f.provider, "provider", "p", "", "AI provider (groq, cerebras)")
	cmd.Flags().StringVarP(&f.model, "model", "m", "", "Override AI model")

	cmd.AddCommand(newConfigCmd())

	return cmd
}

func run(ctx context.Context, f *flags) error {
	out := NewFormatter(os.Stdout, os.Stderr)

	// Check we're in a git repo
	inside, err := git.IsInsideWorkTree()
	if err != nil || !inside {
		return &ExitError{Err: fmt.Errorf("not a git repository"), Code: 2}
	}

	// Resolve config: flags > env > config file
	cfg, err := resolveConfig(f)
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

	// Get the diff
	diff, err := git.StagedDiff()
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

	// Generate commit message
	out.Status("Generating commit message...")
	message, err := provider.GenerateCommitMessage(ctx, diff)
	if err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	// JSON output
	if f.json {
		return out.JSON(message)
	}

	// Dry run — print and exit
	if f.dryRun {
		out.Plain(message)
		return nil
	}

	// Non-interactive mode — commit directly
	if f.noInput {
		return doCommit(out, message)
	}

	// Interactive mode
	out.CommitMessage(message)
	choice, err := prompt.Confirm(os.Stderr, os.Stdin)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("reading input: %v", err), Code: 1}
	}

	switch choice {
	case prompt.Accept:
		return doCommit(out, message)
	case prompt.Edit:
		edited, err := editor.Edit(message)
		if err != nil {
			return &ExitError{Err: err, Code: 1}
		}
		return doCommit(out, edited)
	case prompt.Reject:
		out.Status("Aborted.")
		return nil
	}

	return nil
}

func doCommit(out *Formatter, message string) error {
	hash, err := git.Commit(message)
	if err != nil {
		var gitErr *git.GitError
		if errors.As(err, &gitErr) {
			return &ExitError{Err: fmt.Errorf("commit failed: %s", gitErr.Stderr), Code: 2}
		}
		return &ExitError{Err: err, Code: 2}
	}
	// Extract the title (first line) for the success message
	title := message
	for i, c := range message {
		if c == '\n' {
			title = message[:i]
			break
		}
	}
	out.Success(fmt.Sprintf("[%s] %s", hash, title))
	return nil
}

func resolveConfig(f *flags) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Env var overrides
	if envProvider := os.Getenv("GCA_PROVIDER"); envProvider != "" && cfg.Provider == "" {
		cfg.Provider = envProvider
	}

	// Flag overrides
	if f.provider != "" {
		cfg.Provider = f.provider
	}
	if f.model != "" {
		cfg.Model = f.model
	}

	// Validate provider
	if cfg.Provider == "" {
		return nil, fmt.Errorf("provider not configured. Run: gca config set provider <groq|cerebras>")
	}
	if cfg.Provider != "groq" && cfg.Provider != "cerebras" {
		return nil, fmt.Errorf("invalid provider %q. Choose: groq, cerebras", cfg.Provider)
	}

	// Resolve API key: env var takes precedence over config file
	switch cfg.Provider {
	case "groq":
		if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
		}
	case "cerebras":
		if envKey := os.Getenv("CEREBRAS_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
		}
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key not configured. Run: gca config set api-key <your-key>")
	}

	return cfg, nil
}
