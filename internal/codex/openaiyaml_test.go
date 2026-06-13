package codex

import (
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

func TestMerge_EmptyCreatesMinimalPolicy(t *testing.T) {
	out, err := Merge(nil, false)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	got, present := InferAllowImplicit(out)
	if !present || got != false {
		t.Fatalf("round-trip = (%v, %v), want (false, true)", got, present)
	}
}

func TestMerge_PreservesExistingFields(t *testing.T) {
	existing := []byte(`interface:
  command: do-thing
dependencies:
  - foo
  - bar
policy:
  allow_implicit_invocation: true
  some_other_flag: keep-me
`)

	out, err := Merge(existing, false)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	// Unrelated top-level fields preserved.
	iface, ok := doc["interface"].(map[string]any)
	if !ok || iface["command"] != "do-thing" {
		t.Errorf("interface not preserved: %v", doc["interface"])
	}
	deps, ok := doc["dependencies"].([]any)
	if !ok || len(deps) != 2 || deps[0] != "foo" || deps[1] != "bar" {
		t.Errorf("dependencies not preserved: %v", doc["dependencies"])
	}

	// Unrelated policy field preserved, target field updated.
	policy, ok := doc["policy"].(map[string]any)
	if !ok {
		t.Fatalf("policy missing: %v", doc["policy"])
	}
	if policy["some_other_flag"] != "keep-me" {
		t.Errorf("unrelated policy field not preserved: %v", policy)
	}
	if policy["allow_implicit_invocation"] != false {
		t.Errorf("allow_implicit_invocation = %v, want false", policy["allow_implicit_invocation"])
	}
}

func TestStripManagedPolicy(t *testing.T) {
	tests := []struct {
		name           string
		data           string
		wantContent    bool
		wantContains   string
		wantNotContain string
	}{
		{
			name:        "empty",
			data:        "",
			wantContent: false,
		},
		{
			name:        "policy only collapses to nothing",
			data:        "policy:\n  allow_implicit_invocation: false\n",
			wantContent: false,
		},
		{
			name:           "keeps authored fields, drops managed key",
			data:           "interface:\n  command: do-thing\npolicy:\n  allow_implicit_invocation: true\n",
			wantContent:    true,
			wantContains:   "do-thing",
			wantNotContain: "allow_implicit_invocation",
		},
		{
			name:         "keeps other policy keys",
			data:         "policy:\n  allow_implicit_invocation: true\n  other: keep\n",
			wantContent:  true,
			wantContains: "other: keep",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, hasContent, err := StripManagedPolicy([]byte(tt.data))
			if err != nil {
				t.Fatalf("StripManagedPolicy() error = %v", err)
			}
			if hasContent != tt.wantContent {
				t.Fatalf("hasContent = %v, want %v (rest=%q)", hasContent, tt.wantContent, rest)
			}
			if tt.wantContains != "" && !strings.Contains(string(rest), tt.wantContains) {
				t.Errorf("rest = %q, want contains %q", rest, tt.wantContains)
			}
			if tt.wantNotContain != "" && strings.Contains(string(rest), tt.wantNotContain) {
				t.Errorf("rest = %q, want NOT contains %q", rest, tt.wantNotContain)
			}
		})
	}
}

func TestInferAllowImplicit(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantVal     bool
		wantPresent bool
	}{
		{"empty", "", false, false},
		{"present true", "policy:\n  allow_implicit_invocation: true\n", true, true},
		{"present false", "policy:\n  allow_implicit_invocation: false\n", false, true},
		{"no policy", "interface:\n  command: x\n", false, false},
		{"policy without key", "policy:\n  other: 1\n", false, false},
		{"invalid yaml", "::: not yaml :::", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, present := InferAllowImplicit([]byte(tt.data))
			if present != tt.wantPresent || got != tt.wantVal {
				t.Errorf("InferAllowImplicit() = (%v, %v), want (%v, %v)", got, present, tt.wantVal, tt.wantPresent)
			}
		})
	}
}
