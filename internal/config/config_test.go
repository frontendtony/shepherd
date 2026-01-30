package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "testdata", "config.yaml"))
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.Version)
	assert.Len(t, cfg.Stacks, 2)
	assert.Len(t, cfg.Groups, 2)
	assert.Len(t, cfg.Processes, 3)

	bastion := cfg.Processes["bastion"]
	assert.Equal(t, "Main bastion SSH connection", bastion.Description)
	assert.True(t, bastion.Retry.Enabled)
	assert.Equal(t, 5, bastion.Retry.MaxAttempts)
	assert.Equal(t, 2*time.Second, bastion.Retry.InitialBackoff.Duration())
	assert.Equal(t, 60*time.Second, bastion.Retry.MaxBackoff.Duration())
	assert.Equal(t, 2.0, bastion.Retry.BackoffMultiplier)

	forward := cfg.Processes["staging-forward"]
	assert.Equal(t, []string{"bastion"}, forward.DependsOn)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(path, []byte("{{not yaml"), 0644)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config")
}

func TestValidate_Valid(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "testdata", "config.yaml"))
	require.NoError(t, err)

	err = Validate(cfg)
	assert.NoError(t, err)
}

func TestValidate_MissingProcessInGroup(t *testing.T) {
	cfg := &Config{
		Groups: map[string]Group{
			"g1": {Processes: []string{"nonexistent"}},
		},
		Processes: map[string]Process{},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `group "g1" references undefined process "nonexistent"`)
}

func TestValidate_MissingGroupInStack(t *testing.T) {
	cfg := &Config{
		Stacks: map[string]Stack{
			"s1": {Groups: []string{"nonexistent"}},
		},
		Groups:    map[string]Group{},
		Processes: map[string]Process{},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `stack "s1" references undefined group "nonexistent"`)
}

func TestValidate_DuplicateNames(t *testing.T) {
	cfg := &Config{
		Stacks: map[string]Stack{
			"shared": {Groups: []string{}},
		},
		Groups: map[string]Group{},
		Processes: map[string]Process{
			"shared": {Command: "echo hi"},
		},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `"shared"`)
}

func TestValidate_SelfDependency(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {Command: "echo a", DependsOn: []string{"a"}},
		},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `process "a" depends on itself`)
}

func TestValidate_UndefinedDependency(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {Command: "echo a", DependsOn: []string{"nonexistent"}},
		},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `depends on undefined process "nonexistent"`)
}

func TestValidate_CyclicDependency(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {Command: "echo a", DependsOn: []string{"c"}},
			"b": {Command: "echo b", DependsOn: []string{"a"}},
			"c": {Command: "echo c", DependsOn: []string{"b"}},
		},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle detected")
}

func TestValidate_MissingCommand(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {Description: "no command"},
		},
	}
	applyDefaults(cfg)

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestValidate_InvalidBackoff(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {
				Command: "echo a",
				Retry: RetryConfig{
					Enabled:           true,
					MaxAttempts:       3,
					InitialBackoff:    Duration(30 * time.Second),
					MaxBackoff:        Duration(5 * time.Second),
					BackoffMultiplier: 2,
				},
			},
		},
	}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial_backoff")
	assert.Contains(t, err.Error(), "max_backoff")
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"2s", 2 * time.Second},
		{"60s", 60 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"1m30s", 90 * time.Second},
	}

	for _, tt := range tests {
		var d Duration
		err := d.UnmarshalYAML(func(v interface{}) error {
			*(v.(*string)) = tt.input
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, d.Duration(), "for input %q", tt.input)
	}
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	var d Duration
	err := d.UnmarshalYAML(func(v interface{}) error {
		*(v.(*string)) = "abc"
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")
}

func TestExpandTilde(t *testing.T) {
	home := "/home/testuser"

	assert.Equal(t, home, expandTilde("~", home))
	assert.Equal(t, filepath.Join(home, "projects"), expandTilde("~/projects", home))
	assert.Equal(t, "/absolute/path", expandTilde("/absolute/path", home))
	assert.Equal(t, "relative/path", expandTilde("relative/path", home))
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	assert.Contains(t, path, ".config")
	assert.Contains(t, path, "shepherd")
	assert.Contains(t, path, "config.yaml")
}

func TestGenerateExample(t *testing.T) {
	example := GenerateExample()
	assert.Contains(t, example, "version: 1")
	assert.Contains(t, example, "stacks:")
	assert.Contains(t, example, "groups:")
	assert.Contains(t, example, "processes:")
	assert.Contains(t, example, "depends_on:")
	assert.Contains(t, example, "retry:")
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Processes: map[string]Process{
			"a": {Command: "echo a"},
		},
	}
	applyDefaults(cfg)

	proc := cfg.Processes["a"]
	defaults := DefaultRetryConfig()
	assert.Equal(t, defaults.MaxAttempts, proc.Retry.MaxAttempts)
	assert.Equal(t, defaults.InitialBackoff, proc.Retry.InitialBackoff)
	assert.Equal(t, defaults.MaxBackoff, proc.Retry.MaxBackoff)
	assert.Equal(t, defaults.BackoffMultiplier, proc.Retry.BackoffMultiplier)
}
