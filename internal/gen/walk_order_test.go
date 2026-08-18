package gen

import (
	"slices"
	"testing"
)

func TestWalkOrder_EmptyGraph(t *testing.T) {
	order, cycle := WalkOrder(nil, nil)
	if order != nil {
		t.Errorf("order = %v, want nil", order)
	}
	if cycle != nil {
		t.Errorf("cycle = %v, want nil", cycle)
	}
}

func TestWalkOrder_SingleNodeNoEdges(t *testing.T) {
	order, cycle := WalkOrder([]string{"A"}, nil)
	if !slices.Equal(order, []string{"A"}) {
		t.Errorf("order = %v, want [A]", order)
	}
	if cycle != nil {
		t.Errorf("cycle = %v, want nil", cycle)
	}
}

func TestWalkOrder_LinearDAG(t *testing.T) {
	nodes := []string{"A", "B", "C"}
	edges := [][2]string{{"A", "B"}, {"B", "C"}}

	order, cycle := WalkOrder(nodes, edges)
	if !slices.Equal(order, []string{"A", "B", "C"}) {
		t.Errorf("order = %v, want [A B C]", order)
	}
	if cycle != nil {
		t.Errorf("cycle = %v, want nil", cycle)
	}
}

func TestWalkOrder_DiamondDAGBreaksTiesByDeclarationOrder(t *testing.T) {
	nodes := []string{"A", "B", "C", "D"}
	edges := [][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}}

	order, cycle := WalkOrder(nodes, edges)
	if !slices.Equal(order, []string{"A", "B", "C", "D"}) {
		t.Errorf("order = %v, want [A B C D]", order)
	}
	if cycle != nil {
		t.Errorf("cycle = %v, want nil", cycle)
	}
}

func TestWalkOrder_DirectCycleReturnsDeclarationOrderAndReportsCycle(t *testing.T) {
	nodes := []string{"A", "B"}
	edges := [][2]string{{"A", "B"}, {"B", "A"}}

	order, cycle := WalkOrder(nodes, edges)
	if !slices.Equal(order, []string{"A", "B"}) {
		t.Errorf("order = %v, want [A B]", order)
	}
	if cycle == nil {
		t.Fatal("cycle = nil, want non-nil")
	}
	for _, id := range []string{"A", "B"} {
		if !slices.Contains(cycle, id) {
			t.Errorf("cycle = %v, want it to contain %s", cycle, id)
		}
	}
}

func TestWalkOrder_SelfLoopReturnsDeclarationOrderAndReportsCycle(t *testing.T) {
	nodes := []string{"A"}
	edges := [][2]string{{"A", "A"}}

	order, cycle := WalkOrder(nodes, edges)
	if !slices.Equal(order, []string{"A"}) {
		t.Errorf("order = %v, want [A]", order)
	}
	if cycle == nil {
		t.Fatal("cycle = nil, want non-nil")
	}
	if !slices.Contains(cycle, "A") {
		t.Errorf("cycle = %v, want it to contain A", cycle)
	}
}

func TestWalkOrder_IsDeterministicAcrossRuns(t *testing.T) {
	nodes := []string{"A", "B", "C", "D"}
	edges := [][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}}

	first, _ := WalkOrder(nodes, edges)
	for range 20 {
		next, _ := WalkOrder(nodes, edges)
		if !slices.Equal(first, next) {
			t.Fatalf("order differs between two runs of identical input: %v vs %v", first, next)
		}
	}
}
