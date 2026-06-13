package domain

import "testing"

func TestResolveCodexPolicy(t *testing.T) {
	tests := []struct {
		name        string
		codex       map[string]any
		wantAllow   bool
		wantPresent bool
	}{
		{
			name:        "nil map",
			codex:       nil,
			wantPresent: false,
		},
		{
			name:        "empty map",
			codex:       map[string]any{},
			wantPresent: false,
		},
		{
			name:        "native true",
			codex:       map[string]any{"policy": map[string]any{"allow_implicit_invocation": true}},
			wantAllow:   true,
			wantPresent: true,
		},
		{
			name:        "native false",
			codex:       map[string]any{"policy": map[string]any{"allow_implicit_invocation": false}},
			wantAllow:   false,
			wantPresent: true,
		},
		{
			name:        "alias disable true maps to allow false",
			codex:       map[string]any{"disable-model-invocation": true},
			wantAllow:   false,
			wantPresent: true,
		},
		{
			name:        "alias disable false maps to allow true",
			codex:       map[string]any{"disable-model-invocation": false},
			wantAllow:   true,
			wantPresent: true,
		},
		{
			name: "native wins over conflicting alias",
			codex: map[string]any{
				"policy":                   map[string]any{"allow_implicit_invocation": true},
				"disable-model-invocation": true,
			},
			wantAllow:   true,
			wantPresent: true,
		},
		{
			name:        "policy present but non-bool value falls through",
			codex:       map[string]any{"policy": map[string]any{"allow_implicit_invocation": "yes"}},
			wantPresent: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAllow, gotPresent := ResolveCodexPolicy(tt.codex)
			if gotPresent != tt.wantPresent {
				t.Fatalf("present = %v, want %v", gotPresent, tt.wantPresent)
			}
			if gotAllow != tt.wantAllow {
				t.Errorf("allowImplicit = %v, want %v", gotAllow, tt.wantAllow)
			}
		})
	}
}
