package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath returns the default config file location.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "shepherd", "config.yaml")
}

// Load reads and parses a YAML config file. It applies defaults and expands
// environment variables and ~ in paths.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)
	expandPaths(&cfg)

	return &cfg, nil
}

// Validate checks config for referential integrity and invalid values.
// It returns all validation errors, not just the first.
func Validate(cfg *Config) error {
	var errs []string

	// Collect all names to check for uniqueness across types.
	allNames := make(map[string]string) // name -> type ("stack", "group", "process")

	for name := range cfg.Stacks {
		if existing, ok := allNames[name]; ok {
			errs = append(errs, fmt.Sprintf("name %q is used as both a %s and a stack", name, existing))
		}
		allNames[name] = "stack"
	}
	for name := range cfg.Groups {
		if existing, ok := allNames[name]; ok {
			errs = append(errs, fmt.Sprintf("name %q is used as both a %s and a group", name, existing))
		}
		allNames[name] = "group"
	}
	for name := range cfg.Processes {
		if existing, ok := allNames[name]; ok {
			errs = append(errs, fmt.Sprintf("name %q is used as both a %s and a process", name, existing))
		}
		allNames[name] = "process"
	}

	// Validate stack references.
	for stackName, stack := range cfg.Stacks {
		for _, groupName := range stack.Groups {
			if _, ok := cfg.Groups[groupName]; !ok {
				errs = append(errs, fmt.Sprintf("stack %q references undefined group %q", stackName, groupName))
			}
		}
	}

	// Validate group references.
	for groupName, group := range cfg.Groups {
		for _, procName := range group.Processes {
			if _, ok := cfg.Processes[procName]; !ok {
				errs = append(errs, fmt.Sprintf("group %q references undefined process %q", groupName, procName))
			}
		}
	}

	// Validate dependency references.
	for procName, proc := range cfg.Processes {
		for _, dep := range proc.DependsOn {
			if _, ok := cfg.Processes[dep]; !ok {
				errs = append(errs, fmt.Sprintf("process %q depends on undefined process %q", procName, dep))
			}
			if dep == procName {
				errs = append(errs, fmt.Sprintf("process %q depends on itself", procName))
			}
		}
	}

	// Validate retry config values.
	for procName, proc := range cfg.Processes {
		if proc.Retry.Enabled {
			if proc.Retry.InitialBackoff.Duration() <= 0 {
				errs = append(errs, fmt.Sprintf("process %q: initial_backoff must be positive", procName))
			}
			if proc.Retry.MaxBackoff.Duration() <= 0 {
				errs = append(errs, fmt.Sprintf("process %q: max_backoff must be positive", procName))
			}
			if proc.Retry.InitialBackoff.Duration() > proc.Retry.MaxBackoff.Duration() {
				errs = append(errs, fmt.Sprintf("process %q: initial_backoff (%s) must be <= max_backoff (%s)",
					procName, proc.Retry.InitialBackoff.Duration(), proc.Retry.MaxBackoff.Duration()))
			}
			if proc.Retry.BackoffMultiplier < 1 {
				errs = append(errs, fmt.Sprintf("process %q: backoff_multiplier must be >= 1", procName))
			}
		}

		if proc.Command == "" {
			errs = append(errs, fmt.Sprintf("process %q: command is required", procName))
		}
	}

	// Detect dependency cycles.
	if err := detectCycles(cfg); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// detectCycles uses Kahn's algorithm to detect cycles in the dependency graph.
func detectCycles(cfg *Config) error {
	// Build in-degree map.
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // dep -> processes that depend on it

	for name := range cfg.Processes {
		inDegree[name] = 0
	}
	for name, proc := range cfg.Processes {
		for _, dep := range proc.DependsOn {
			if _, ok := cfg.Processes[dep]; ok {
				inDegree[name]++
				dependents[dep] = append(dependents[dep], name)
			}
		}
	}

	// Start with all nodes that have no dependencies.
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(cfg.Processes) {
		// Find the processes involved in cycles.
		var cycleNodes []string
		for name, degree := range inDegree {
			if degree > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		return fmt.Errorf("dependency cycle detected involving: %s", strings.Join(cycleNodes, ", "))
	}
	return nil
}

// GenerateExample returns a commented example config YAML string.
func GenerateExample() string {
	return `# Shepherd configuration
# See: https://github.com/frontendtony/shepherd
version: 1

# Stacks: named collections of groups for quick startup
# Usage: shepherd <stack-name>
stacks:
  dev:
    description: "Full development environment"
    groups: [tunnels, database]

# Groups: logical groupings of related processes
groups:
  tunnels:
    description: "SSH tunnels and bastion connections"
    processes: [bastion]
  database:
    description: "Database connections"
    processes: [db-tunnel]

# Process definitions
processes:
  bastion:
    description: "Main bastion SSH connection"
    command: "ssh -N -o ServerAliveInterval=60 -L 2222:internal-jump:22 bastion.example.com"
    retry:
      enabled: true
      max_attempts: 5
      initial_backoff: 2s
      max_backoff: 60s
      backoff_multiplier: 2

  db-tunnel:
    description: "Database tunnel through bastion"
    command: "ssh -N -L 5432:db.internal:5432 -p 2222 localhost"
    depends_on: [bastion]
    retry:
      enabled: true
      max_attempts: 3
      initial_backoff: 5s
      max_backoff: 30s
      backoff_multiplier: 2
`
}

func applyDefaults(cfg *Config) {
	if cfg.Stacks == nil {
		cfg.Stacks = make(map[string]Stack)
	}
	if cfg.Groups == nil {
		cfg.Groups = make(map[string]Group)
	}
	if cfg.Processes == nil {
		cfg.Processes = make(map[string]Process)
	}

	defaults := DefaultRetryConfig()
	for name, proc := range cfg.Processes {
		if proc.Retry.MaxAttempts == 0 && !proc.Retry.Enabled {
			proc.Retry.MaxAttempts = defaults.MaxAttempts
		}
		if proc.Retry.InitialBackoff == 0 {
			proc.Retry.InitialBackoff = defaults.InitialBackoff
		}
		if proc.Retry.MaxBackoff == 0 {
			proc.Retry.MaxBackoff = defaults.MaxBackoff
		}
		if proc.Retry.BackoffMultiplier == 0 {
			proc.Retry.BackoffMultiplier = defaults.BackoffMultiplier
		}
		cfg.Processes[name] = proc
	}
}

func expandPaths(cfg *Config) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	for name, proc := range cfg.Processes {
		proc.WorkingDir = expandTilde(proc.WorkingDir, home)
		proc.WorkingDir = os.ExpandEnv(proc.WorkingDir)

		for k, v := range proc.Env {
			proc.Env[k] = expandTilde(v, home)
			proc.Env[k] = os.ExpandEnv(proc.Env[k])
		}
		cfg.Processes[name] = proc
	}
}

func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
