package command

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)

	configGetCmd.Flags().StringP("host", "h", "", "Get per-host setting")
	configSetCmd.Flags().StringP("host", "h", "", "Set per-host setting")

	// TODO reveal and add usage once we properly support multiple hosts
	_ = configGetCmd.Flags().MarkHidden("host")
	// TODO reveal and add usage once we properly support multiple hosts
	_ = configSetCmd.Flags().MarkHidden("host")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration for gh",
	Long: `Display or change configuration settings for gh.

Current respected settings:
- git_protocol: "https" or "ssh". Default is "https".
- editor: if unset, defaults to environment variables.
`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print the value of a given configuration key",
	Example: heredoc.Doc(`
	$ gh config get git_protocol
	https
	`),
	Args: cobra.ExactArgs(1),
	RunE: configGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update configuration with a value for the given key",
	Example: heredoc.Doc(`
	$ gh config set editor vim
	$ gh config set editor "code --wait"
	`),
	Args: cobra.ExactArgs(2),
	RunE: configSet,
}

func configGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	hostname, err := cmd.Flags().GetString("host")
	if err != nil {
		return err
	}

	ctx := contextForCommand(cmd)

	cfg, err := ctx.Config()
	if err != nil {
		return err
	}

	val, err := cfg.Get(hostname, key)
	if err != nil {
		return err
	}

	if val != "" {
		out := colorableOut(cmd)
		fmt.Fprintf(out, "%s\n", val)
	}

	return nil
}

func configSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	hostname, err := cmd.Flags().GetString("host")
	if err != nil {
		return err
	}

	ctx := contextForCommand(cmd)

	cfg, err := ctx.Config()
	if err != nil {
		return err
	}

	err = cfg.Set(hostname, key, value)
	if err != nil {
		return fmt.Errorf("failed to set %q to %q: %w", key, value, err)
	}

	err = cfg.Write()
	if err != nil {
		return fmt.Errorf("failed to write config to disk: %w", err)
	}

	return nil
}
