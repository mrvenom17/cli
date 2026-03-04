package perf

import (
	"context"
	"testing"
	"time"
)

func TestStart_CreatesRootSpan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctx, span := Start(ctx, "test_op")
	if span == nil {
		t.Fatal("Start() returned nil span")
	}
	if span.name != "test_op" {
		t.Errorf("span.name = %q, want %q", span.name, "test_op")
	}
	if span.parent != nil {
		t.Error("root span should have nil parent")
	}

	got := spanFromContext(ctx)
	if got != span {
		t.Error("spanFromContext should return the span set by Start")
	}

	span.End()
}

func TestStart_NestsChildSpan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctx, parent := Start(ctx, "parent")
	_, child := Start(ctx, "child")

	if child.parent != parent {
		t.Error("child span should reference parent")
	}
	if len(parent.children) != 1 {
		t.Fatalf("parent should have 1 child, got %d", len(parent.children))
	}
	if parent.children[0] != child {
		t.Error("parent.children[0] should be the child span")
	}

	child.End()
	parent.End()
}

func TestEnd_RecordsDuration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, span := Start(ctx, "timed_op")
	time.Sleep(10 * time.Millisecond)
	span.End()

	if span.duration < 10*time.Millisecond {
		t.Errorf("span.duration = %v, want >= 10ms", span.duration)
	}
}

func TestEnd_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, span := Start(ctx, "double_end")
	time.Sleep(10 * time.Millisecond)
	span.End()

	firstDuration := span.duration

	time.Sleep(10 * time.Millisecond)
	span.End()

	if span.duration != firstDuration {
		t.Errorf("second End() changed duration from %v to %v", firstDuration, span.duration)
	}
}

func TestEnd_AutoEndsChildren(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctx, parent := Start(ctx, "parent")
	_, child := Start(ctx, "child")

	time.Sleep(10 * time.Millisecond)
	parent.End()

	if !child.ended {
		t.Error("child should be auto-ended when parent ends")
	}
	if child.duration == 0 {
		t.Error("child should have non-zero duration after auto-end")
	}
}

func TestMeasure_TimesFunction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, span := Start(ctx, "parent")

	span.Measure("substep", func() {
		time.Sleep(10 * time.Millisecond)
	})

	span.End()

	if len(span.children) != 1 {
		t.Fatalf("expected 1 child from Measure, got %d", len(span.children))
	}
	child := span.children[0]
	if child.name != "substep" {
		t.Errorf("child.name = %q, want %q", child.name, "substep")
	}
	if child.duration < 10*time.Millisecond {
		t.Errorf("child.duration = %v, want >= 10ms", child.duration)
	}
}

func TestSpanFromContext_ReturnsNilWhenEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if got := spanFromContext(ctx); got != nil {
		t.Errorf("spanFromContext on empty context = %v, want nil", got)
	}
}

func TestStart_MultipleChildren(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctx, parent := Start(ctx, "parent")

	ctx2, child1 := Start(ctx, "child1")
	_ = ctx2
	child1.End()

	ctx3, child2 := Start(ctx, "child2")
	_ = ctx3
	child2.End()

	parent.End()

	if len(parent.children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(parent.children))
	}
	if parent.children[0].name != "child1" {
		t.Errorf("first child name = %q, want %q", parent.children[0].name, "child1")
	}
	if parent.children[1].name != "child2" {
		t.Errorf("second child name = %q, want %q", parent.children[1].name, "child2")
	}
}
