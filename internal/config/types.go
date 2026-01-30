package config

import (
	"fmt"
	"time"
)

// Duration wraps time.Duration to support YAML string unmarshaling (e.g., "2s", "500ms").
type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

type Config struct {
	Version   int                `yaml:"version"`
	Stacks    map[string]Stack   `yaml:"stacks"`
	Groups    map[string]Group   `yaml:"groups"`
	Processes map[string]Process `yaml:"processes"`
}

type Stack struct {
	Description string   `yaml:"description"`
	Groups      []string `yaml:"groups"`
}

type Group struct {
	Description string   `yaml:"description"`
	Processes   []string `yaml:"processes"`
}

type Process struct {
	Description string            `yaml:"description"`
	Command     string            `yaml:"command"`
	WorkingDir  string            `yaml:"working_dir"`
	Env         map[string]string `yaml:"env"`
	DependsOn   []string          `yaml:"depends_on"`
	Retry       RetryConfig       `yaml:"retry"`
}

type RetryConfig struct {
	Enabled           bool     `yaml:"enabled"`
	MaxAttempts       int      `yaml:"max_attempts"`
	InitialBackoff    Duration `yaml:"initial_backoff"`
	MaxBackoff        Duration `yaml:"max_backoff"`
	BackoffMultiplier float64  `yaml:"backoff_multiplier"`
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:           false,
		MaxAttempts:       3,
		InitialBackoff:    Duration(2 * time.Second),
		MaxBackoff:        Duration(60 * time.Second),
		BackoffMultiplier: 2.0,
	}
}
