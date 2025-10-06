package api

import "testing"

// Verify NewTaskResponse sets defaults and empty outcomes
func TestNewTaskResponseDefaults(t *testing.T) {
	r := NewTaskResponse()
	if r == nil {
		t.Fatalf("expected TaskResponse, got nil")
	}
	if r.Data.Type != "task-results" {
		t.Fatalf("unexpected Data.Type: %s", r.Data.Type)
	}
	if r.Data.Relationships == nil {
		t.Fatalf("expected Relationships to be initialized")
	}
	if r.Data.Relationships.Outcomes.Data == nil || len(r.Data.Relationships.Outcomes.Data) != 0 {
		t.Fatalf("expected no outcomes by default")
	}
}

// Ensure fluent chaining returns the same pointer and sets values as expected
func TestFluentChainingPassFlow(t *testing.T) {
	base := NewTaskResponse()
	r := base.
		AddOutcome("o1", "desc1", "body1", "https://example.com/1", "OK", TagLevelInfo).
		AddOutcome("o2", "", "", "", "WARN", TagLevelWarning).
		SetResult(TaskPassed, "All good").
		WithUrl("https://example.com/task")

	// chaining should mutate and return the same pointer
	if r != base {
		t.Fatalf("expected chaining to return the same pointer")
	}

	if got, want := r.Data.Attributes.Status, TaskPassed; got != want {
		t.Fatalf("unexpected status: %v", got)
	}
	if got, want := r.Data.Attributes.Message, "All good"; got != want {
		t.Fatalf("unexpected message: %q", got)
	}
	if got, want := r.Data.Attributes.URL, "https://example.com/task"; got != want {
		t.Fatalf("unexpected url: %q", got)
	}

	if len(r.Data.Relationships.Outcomes.Data) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(r.Data.Relationships.Outcomes.Data))
	}

	o1 := r.Data.Relationships.Outcomes.Data[0]
	if o1.Type != "task-result-outcomes" || o1.Attributes.OutcomeID != "o1" || o1.Attributes.Description != "desc1" {
		t.Fatalf("unexpected first outcome: %+v", o1)
	}
	if o1.Attributes.Body != "body1" || o1.Attributes.URL != "https://example.com/1" {
		t.Fatalf("unexpected first outcome body/url: %+v", o1.Attributes)
	}
	if len(o1.Attributes.Tags.Status) != 1 || o1.Attributes.Tags.Status[0].Label != "OK" || o1.Attributes.Tags.Status[0].Level != TagLevelInfo {
		t.Fatalf("unexpected first outcome tag: %+v", o1.Attributes.Tags.Status)
	}

	o2 := r.Data.Relationships.Outcomes.Data[1]
	if len(o2.Attributes.Tags.Status) != 1 || o2.Attributes.Tags.Status[0].Label != "WARN" || o2.Attributes.Tags.Status[0].Level != TagLevelWarning {
		t.Fatalf("unexpected second outcome tag: %+v", o2.Attributes.Tags.Status)
	}
}

// Validate WithUrl only accepts http/https and leaves URL unset otherwise
func TestWithUrlValidation(t *testing.T) {
	// invalid scheme should not set URL
	r1 := NewTaskResponse().WithUrl("ftp://not-allowed")
	if r1.Data.Attributes.URL != "" {
		t.Fatalf("expected URL to remain empty for invalid scheme, got %q", r1.Data.Attributes.URL)
	}

	// missing scheme should not set URL
	r2 := NewTaskResponse().WithUrl("example.com/no-scheme")
	if r2.Data.Attributes.URL != "" {
		t.Fatalf("expected URL to remain empty for missing scheme, got %q", r2.Data.Attributes.URL)
	}

	// http should set
	r3 := NewTaskResponse().WithUrl("http://ok")
	if r3.Data.Attributes.URL != "http://ok" {
		t.Fatalf("expected URL to be set for http scheme, got %q", r3.Data.Attributes.URL)
	}
}

// IsPassed behavior across common cases
func TestIsPassed(t *testing.T) {
	// no outcomes -> passed
	if !NewTaskResponse().IsPassed() {
		t.Fatalf("expected IsPassed true for zero outcomes")
	}

	// only info/warning -> passed
	rOK := NewTaskResponse().
		AddOutcome("a", "", "", "", "ok", TagLevelInfo).
		AddOutcome("b", "", "", "", "warn", TagLevelWarning)
	if !rOK.IsPassed() {
		t.Fatalf("expected IsPassed true when no error tags present")
	}

	// any error -> failed
	rErr := NewTaskResponse().AddOutcome("c", "", "", "", "err", TagLevelError)
	if rErr.IsPassed() {
		t.Fatalf("expected IsPassed false when an error tag is present")
	}
}

func TestJsonApiMediaTypeHeaderConstant(t *testing.T) {
	if JsonApiMediaTypeHeader != "application/vnd.api+json" {
		t.Fatalf("unexpected JsonApiMediaTypeHeader: %q", JsonApiMediaTypeHeader)
	}
}
