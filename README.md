# Shepherd

A TUI process orchestrator for development environments. Shepherd manages long-running processes like SSH tunnels, port forwards, and database proxies with dependency management, automatic retries, and a terminal interface for monitoring.

## Features

- **Process management** - Start, stop, and restart processes with keyboard shortcuts
- **Dependency resolution** - Processes start in dependency order; dependents stop when a dependency fails
- **Automatic retries** - Exponential backoff with configurable limits for crashed processes
- **PTY output capture** - Preserves ANSI colors from process output
- **Grouped process list** - Organize processes into groups and stacks
- **Live log viewer** - Scrollable, auto-following log panel with fullscreen mode
- **Hot config reload** - Send SIGHUP to reload configuration without restarting

## Installation

### From source

Requires Go 1.21+.

```bash
go install github.com/frontendtony/shepherd@latest
```

### Build locally

```bash
git clone https://github.com/frontendtony/shepherd.git
cd shepherd
make build
```

## Quick Start

Run `shepherd` with no arguments. On first run, an example config is created at `~/.config/shepherd/config.yaml`:

```bash
shepherd
```

Edit the config to define your processes, then run again. Optionally auto-start a stack, group, or process by name:

```bash
shepherd dev          # start the "dev" stack
shepherd bastion      # start a single process
```

## Configuration

Config file location: `~/.config/shepherd/config.yaml` (override with `--config`).

```yaml
version: 1

stacks:
  dev:
    description: "Full development environment"
    groups: [tunnels, database]

groups:
  tunnels:
    description: "SSH tunnels"
    processes: [bastion]
  database:
    description: "Database connections"
    processes: [db-tunnel]

processes:
  bastion:
    description: "Bastion SSH connection"
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
```

### Process options

| Field | Description |
|---|---|
| `command` | Shell command to run (executed via `sh -c`) |
| `description` | Human-readable description |
| `working_dir` | Working directory (supports `~` and `$ENV_VAR`) |
| `env` | Environment variables (map of key-value pairs) |
| `depends_on` | List of process names this process depends on |
| `retry.enabled` | Enable automatic retries on failure |
| `retry.max_attempts` | Maximum retry attempts (default: 3) |
| `retry.initial_backoff` | Initial backoff duration (default: 2s) |
| `retry.max_backoff` | Maximum backoff duration (default: 60s) |
| `retry.backoff_multiplier` | Backoff multiplier (default: 2.0) |

### Validation

The config is validated on load. Shepherd checks for:
- Duplicate names across stacks, groups, and processes
- Missing references (groups referencing non-existent processes, etc.)
- Circular dependencies
- Invalid retry values

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Expand/collapse group |
| `Tab` | Switch panel focus |
| `l` | Focus log panel |
| `f` | Toggle fullscreen logs |

### Process control

| Key | Action |
|---|---|
| `s` | Start selected process |
| `x` | Stop selected process |
| `r` | Restart selected process |
| `g` | Start all in group |
| `G` | Stop all in group |
| `a` | Start all processes |
| `X` | Stop all processes |

### Other

| Key | Action |
|---|---|
| `?` | Toggle help overlay |
| `q` | Quit (confirms if processes are running) |

## Signals

| Signal | Action |
|---|---|
| `SIGHUP` | Reload configuration |
| `SIGINT` / `SIGTERM` | Graceful shutdown (stops all processes) |

## CLI flags

```
shepherd [name] [flags]

Flags:
  -c, --config string   path to config file (default "~/.config/shepherd/config.yaml")
  -v, --verbose          enable debug logging
  -h, --help             help for shepherd
```

## Requirements

- macOS or Linux
- Go 1.21+ (to build from source)
