package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "shepherd [name]",
	Short: "A process orchestrator for development environments",
	Long: `Shepherd keeps watch over your processes, herding them together,
ensuring none stray, and bringing back any that wander off.

Run without arguments to open the TUI. Optionally pass a stack,
group, or process name to auto-start it on launch.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Load config, create process manager, launch TUI
		fmt.Println("shepherd - process orchestrator")
		fmt.Println("TUI not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file (default: ~/.config/shepherd/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
