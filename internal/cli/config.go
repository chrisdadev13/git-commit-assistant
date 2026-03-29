package cli

import (
	"fmt"

	"gca/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage gca configuration",
		Long:  "Get, set, and list gca configuration values (provider, api-key, model).",
	}

	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigPathCmd())

	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Long:  "Valid keys: provider, api-key, model",
		Example: `  gca config set provider groq
  gca config set api-key gsk_abc123
  gca config set model llama-3.3-70b-versatile`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			if !config.IsValidKey(key) {
				return &ExitError{
					Err:  fmt.Errorf("unknown config key %q. Valid keys: provider, api-key, model", key),
					Code: 1,
				}
			}

			cfg, err := config.Load()
			if err != nil {
				return &ExitError{Err: err, Code: 1}
			}

			if err := cfg.Set(key, value); err != nil {
				return &ExitError{Err: err, Code: 1}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Set %s = %s\n", key, value)
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			if !config.IsValidKey(key) {
				return &ExitError{
					Err:  fmt.Errorf("unknown config key %q. Valid keys: provider, api-key, model", key),
					Code: 1,
				}
			}

			cfg, err := config.Load()
			if err != nil {
				return &ExitError{Err: err, Code: 1}
			}

			val := cfg.Get(key)
			if val == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "(not set)")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), val)
			}
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return &ExitError{Err: err, Code: 1}
			}

			out := cmd.OutOrStdout()

			provider := cfg.Provider
			if provider == "" {
				provider = "(not set)"
			}
			fmt.Fprintf(out, "provider = %s\n", provider)

			apiKey := "(not set)"
			if cfg.APIKey != "" {
				apiKey = config.MaskKey(cfg.APIKey)
			}
			fmt.Fprintf(out, "api-key  = %s\n", apiKey)

			model := cfg.Model
			if model == "" {
				model = "(default)"
			}
			fmt.Fprintf(out, "model    = %s\n", model)

			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print config file path",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), config.Path())
		},
	}
}
