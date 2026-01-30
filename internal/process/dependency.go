package process

import (
	"fmt"
	"strings"

	"github.com/frontendtony/shepherd/internal/config"
)

// DependencyGraph tracks process dependencies for start ordering and cascading stops.
type DependencyGraph struct {
	// forward: process -> its dependencies (what it depends on)
	forward map[string][]string
	// reverse: process -> processes that depend on it
	reverse map[string][]string
	// all known process names
	nodes map[string]bool
}

// NewDependencyGraph builds a dependency graph from config.
func NewDependencyGraph(cfg *config.Config) *DependencyGraph {
	g := &DependencyGraph{
		forward: make(map[string][]string),
		reverse: make(map[string][]string),
		nodes:   make(map[string]bool),
	}

	for name, proc := range cfg.Processes {
		g.nodes[name] = true
		g.forward[name] = proc.DependsOn
		for _, dep := range proc.DependsOn {
			g.reverse[dep] = append(g.reverse[dep], name)
		}
	}

	return g
}

// Validate checks for cycles using Kahn's algorithm.
func (g *DependencyGraph) Validate() error {
	inDegree := make(map[string]int)
	for name := range g.nodes {
		inDegree[name] = len(g.forward[name])
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, dep := range g.reverse[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(g.nodes) {
		var cycleNodes []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		return fmt.Errorf("dependency cycle detected involving: %s", strings.Join(cycleNodes, ", "))
	}
	return nil
}

// StartOrder returns a topological ordering of the given targets and all their
// transitive dependencies. Dependencies come first in the returned slice.
func (g *DependencyGraph) StartOrder(targets []string) ([]string, error) {
	// Collect all required nodes (targets + transitive deps).
	required := make(map[string]bool)
	var collectDeps func(name string)
	collectDeps = func(name string) {
		if required[name] {
			return
		}
		required[name] = true
		for _, dep := range g.forward[name] {
			collectDeps(dep)
		}
	}
	for _, t := range targets {
		if !g.nodes[t] {
			return nil, fmt.Errorf("unknown process: %s", t)
		}
		collectDeps(t)
	}

	// Topological sort of required nodes using Kahn's algorithm.
	inDegree := make(map[string]int)
	for name := range required {
		count := 0
		for _, dep := range g.forward[name] {
			if required[dep] {
				count++
			}
		}
		inDegree[name] = count
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, dep := range g.reverse[node] {
			if !required[dep] {
				continue
			}
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(order) != len(required) {
		return nil, fmt.Errorf("dependency cycle detected")
	}

	return order, nil
}

// StopOrder returns the reverse of StartOrder — dependents come first
// so they are stopped before their dependencies.
func (g *DependencyGraph) StopOrder(targets []string) ([]string, error) {
	order, err := g.StartOrder(targets)
	if err != nil {
		return nil, err
	}
	// Reverse in place.
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order, nil
}

// Dependents returns all processes that directly or transitively depend on the
// given process (i.e., processes that must be stopped if name is stopped).
func (g *DependencyGraph) Dependents(name string) []string {
	visited := make(map[string]bool)
	var result []string

	var walk func(n string)
	walk = func(n string) {
		for _, dep := range g.reverse[n] {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				walk(dep)
			}
		}
	}

	walk(name)
	return result
}

// Dependencies returns all processes that the given process directly or
// transitively depends on (i.e., processes that must be started before name).
func (g *DependencyGraph) Dependencies(name string) []string {
	visited := make(map[string]bool)
	var result []string

	var walk func(n string)
	walk = func(n string) {
		for _, dep := range g.forward[n] {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				walk(dep)
			}
		}
	}

	walk(name)
	return result
}
