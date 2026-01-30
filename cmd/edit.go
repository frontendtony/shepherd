package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config file in your editor",
	Long:  `Opens the shepherd config file in $EDITOR (falls back to nano).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPath
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}

		c := exec.Command(editor, cfgPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("opening editor: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
