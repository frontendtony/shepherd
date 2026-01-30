package process

import (
	"sort"
	"testing"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildGraph(procs map[string]config.Process) *DependencyGraph {
	cfg := &config.Config{Processes: procs}
	return NewDependencyGraph(cfg)
}

func TestDependencyGraph_LinearChain(t *testing.T) {
	// C <- B <- A (A depends on B, B depends on C)
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a", DependsOn: []string{"B"}},
		"B": {Command: "b", DependsOn: []string{"C"}},
		"C": {Command: "c"},
	})

	require.NoError(t, g.Validate())

	order, err := g.StartOrder([]string{"A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"C", "B", "A"}, order)
}

func TestDependencyGraph_Diamond(t *testing.T) {
	// D depends on B and C, both depend on A
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c", DependsOn: []string{"A"}},
		"D": {Command: "d", DependsOn: []string{"B", "C"}},
	})

	require.NoError(t, g.Validate())

	order, err := g.StartOrder([]string{"D"})
	require.NoError(t, err)

	// A must be first, D must be last. B and C can be in any order.
	assert.Equal(t, "A", order[0])
	assert.Equal(t, "D", order[3])
	middle := order[1:3]
	sort.Strings(middle)
	assert.Equal(t, []string{"B", "C"}, middle)
}

func TestDependencyGraph_CycleDetected(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a", DependsOn: []string{"C"}},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c", DependsOn: []string{"B"}},
	})

	err := g.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle detected")
}

func TestDependencyGraph_SelfCycle(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a", DependsOn: []string{"A"}},
	})

	err := g.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle detected")
}

func TestDependencyGraph_NoDependencies(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b"},
		"C": {Command: "c"},
	})

	require.NoError(t, g.Validate())

	order, err := g.StartOrder([]string{"A", "B", "C"})
	require.NoError(t, err)
	assert.Len(t, order, 3)

	// All three should be present.
	sorted := make([]string, len(order))
	copy(sorted, order)
	sort.Strings(sorted)
	assert.Equal(t, []string{"A", "B", "C"}, sorted)
}

func TestDependencyGraph_DisconnectedComponents(t *testing.T) {
	// Two independent chains: A->B, C->D
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a", DependsOn: []string{"B"}},
		"B": {Command: "b"},
		"C": {Command: "c", DependsOn: []string{"D"}},
		"D": {Command: "d"},
	})

	require.NoError(t, g.Validate())

	// Request just one chain.
	order, err := g.StartOrder([]string{"A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"B", "A"}, order)

	// Request both.
	order, err = g.StartOrder([]string{"A", "C"})
	require.NoError(t, err)
	assert.Len(t, order, 4)
}

func TestDependencyGraph_TargetSubset(t *testing.T) {
	// B depends on A. Requesting just B should include A.
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c"},
	})

	order, err := g.StartOrder([]string{"B"})
	require.NoError(t, err)
	assert.Equal(t, []string{"A", "B"}, order)
}

func TestDependencyGraph_UnknownTarget(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
	})

	_, err := g.StartOrder([]string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown process")
}

func TestDependencyGraph_Dependents(t *testing.T) {
	// B and C both depend on A
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c", DependsOn: []string{"A"}},
	})

	deps := g.Dependents("A")
	sort.Strings(deps)
	assert.Equal(t, []string{"B", "C"}, deps)
}

func TestDependencyGraph_Dependents_Transitive(t *testing.T) {
	// C depends on B, B depends on A
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c", DependsOn: []string{"B"}},
	})

	deps := g.Dependents("A")
	sort.Strings(deps)
	assert.Equal(t, []string{"B", "C"}, deps)
}

func TestDependencyGraph_Dependencies(t *testing.T) {
	// C depends on B, B depends on A
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
		"C": {Command: "c", DependsOn: []string{"B"}},
	})

	deps := g.Dependencies("C")
	sort.Strings(deps)
	assert.Equal(t, []string{"A", "B"}, deps)
}

func TestDependencyGraph_Dependencies_None(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
	})

	deps := g.Dependencies("A")
	assert.Empty(t, deps)
}

func TestDependencyGraph_StopOrder(t *testing.T) {
	// C <- B <- A (start: C, B, A; stop: A, B, C)
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a", DependsOn: []string{"B"}},
		"B": {Command: "b", DependsOn: []string{"C"}},
		"C": {Command: "c"},
	})

	order, err := g.StopOrder([]string{"A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"A", "B", "C"}, order)
}

func TestDependencyGraph_Dependents_LeafNode(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
	})

	deps := g.Dependents("B")
	assert.Empty(t, deps)
}

func TestDependencyGraph_StartOrder_SingleNode(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
	})

	order, err := g.StartOrder([]string{"A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"A"}, order)
}

func TestDependencyGraph_Validate_NoCycle(t *testing.T) {
	g := buildGraph(map[string]config.Process{
		"A": {Command: "a"},
		"B": {Command: "b", DependsOn: []string{"A"}},
	})

	assert.NoError(t, g.Validate())
}
