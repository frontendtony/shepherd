package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/frontendtony/shepherd/internal/config"
	"github.com/frontendtony/shepherd/internal/process"
	"github.com/frontendtony/shepherd/internal/tui"
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
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPath
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		// First-run: generate example config if none exists.
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			dir := filepath.Dir(cfgPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}
			if err := os.WriteFile(cfgPath, []byte(config.GenerateExample()), 0o644); err != nil {
				return fmt.Errorf("writing example config: %w", err)
			}
			fmt.Printf("Created example config at %s\nEdit it and run shepherd again.\n", cfgPath)
			return nil
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if err := config.Validate(cfg); err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle OS signals for graceful shutdown.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		mgr, err := process.NewProcessManager(ctx, cfg)
		if err != nil {
			return fmt.Errorf("creating process manager: %w", err)
		}
		defer mgr.Shutdown()

		var autoStart string
		if len(args) == 1 {
			autoStart = args[0]
		}

		model := tui.NewModel(mgr, cfg, autoStart)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file (default: ~/.config/shepherd/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
