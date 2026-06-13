package skillmd

import (
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func TestParse(t *testing.T) {
	content := "---\nname: Test Skill\ndescription: \"A: B\"\ndisable-model-invocation: true\n---\n\n# Test\nBody\n"
	parsed := Parse(content)

	if got, want := parsed.Fields["name"], "Test Skill"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := parsed.Fields["description"], "A: B"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
	// YAML-typed values: a bool stays a bool, not a string.
	if got, want := parsed.Fields["disable-model-invocation"], true; got != want {
		t.Fatalf("disable-model-invocation = %v (%T), want %v", got, got, want)
	}
	if !strings.Contains(parsed.Body, "# Test") {
		t.Fatalf("body = %q, want heading", parsed.Body)
	}
}

func TestParse_NoFrontmatter(t *testing.T) {
	content := "# Test\nBody\n"
	parsed := Parse(content)
	if len(parsed.Fields) != 0 {
		t.Fatalf("fields = %v, want empty", parsed.Fields)
	}
	if parsed.Body != content {
		t.Fatalf("body = %q, want %q", parsed.Body, content)
	}
}

func TestHeading(t *testing.T) {
	if got, want := Heading("intro\n# Test Skill\n"), "Test Skill"; got != want {
		t.Fatalf("Heading() = %q, want %q", got, want)
	}
}

func TestBuildFrontmatter(t *testing.T) {
	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill.",
		Claude: map[string]any{
			"allowed-tools": "Read, Grep",
		},
	}

	got := BuildFrontmatter(skill, domain.TargetClaude)
	if !strings.Contains(got, "name: test-skill") {
		t.Fatalf("frontmatter missing name:\n%s", got)
	}
	// yaml.v3 leaves scalars unquoted when no quoting is required.
	if !strings.Contains(got, "description: A test skill.") {
		t.Fatalf("frontmatter missing description:\n%s", got)
	}
	if !strings.Contains(got, "allowed-tools: Read, Grep") {
		t.Fatalf("frontmatter missing target meta:\n%s", got)
	}
}

func TestBuildFrontmatter_OrdersNameAndDescriptionFirst(t *testing.T) {
	skill := domain.Skill{
		Name:        "test-skill",
		Description: "desc",
		Claude:      map[string]any{"allowed-tools": "Read"},
	}

	got := BuildFrontmatter(skill, domain.TargetClaude)
	want := "name: test-skill\ndescription: desc\nallowed-tools: Read\n"
	if got != want {
		t.Fatalf("BuildFrontmatter() =\n%q\nwant\n%q", got, want)
	}
}

func TestBuildFrontmatter_QuotesDescription(t *testing.T) {
	skill := domain.Skill{
		Name:        "git-reorganize",
		Description: "Does stuff. Conversational: analyzes things",
	}

	got := BuildFrontmatter(skill, domain.TargetClaude)
	// A colon-space forces quoting; yaml.v3 uses single quotes here.
	want := `description: 'Does stuff. Conversational: analyzes things'`
	if !strings.Contains(got, want) {
		t.Fatalf("expected line %q in:\n%s", want, got)
	}
}

func TestBuildFrontmatter_PreservesBoolMeta(t *testing.T) {
	skill := domain.Skill{
		Name:        "test",
		Description: "desc",
		Claude: map[string]any{
			"disable-model-invocation": true,
		},
	}

	got := BuildFrontmatter(skill, domain.TargetClaude)
	want := "disable-model-invocation: true\n"
	if !strings.Contains(got, want) {
		t.Fatalf("expected unquoted bool %q in:\n%s", want, got)
	}
}

func TestBuildFrontmatter_QuotesMetaValues(t *testing.T) {
	skill := domain.Skill{
		Name:        "test",
		Description: "desc",
		Codex: map[string]any{
			"compatibility": "Requires git: yes",
		},
	}

	got := BuildFrontmatter(skill, domain.TargetCodex)
	want := `compatibility: 'Requires git: yes'`
	if !strings.Contains(got, want) {
		t.Fatalf("expected line %q in:\n%s", want, got)
	}
}

func TestBuildFrontmatter_FiltersCodexPolicyKeys(t *testing.T) {
	skill := domain.Skill{
		Name:        "test",
		Description: "desc",
		Codex: map[string]any{
			"policy":                   map[string]any{"allow_implicit_invocation": false},
			"disable-model-invocation": true,
			"compatibility":            "ok",
		},
	}

	got := BuildFrontmatter(skill, domain.TargetCodex)
	if strings.Contains(got, "policy") {
		t.Errorf("codex frontmatter should not contain policy:\n%s", got)
	}
	if strings.Contains(got, "disable-model-invocation") {
		t.Errorf("codex frontmatter should not contain disable-model-invocation:\n%s", got)
	}
	if !strings.Contains(got, "compatibility: ok") {
		t.Errorf("codex frontmatter should keep non-policy meta:\n%s", got)
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"with frontmatter",
			"---\nname: test\ndescription: foo\n---\n\n# Heading\nBody\n",
			"# Heading\nBody\n",
		},
		{
			"no frontmatter",
			"# Heading\nBody\n",
			"# Heading\nBody\n",
		},
		{
			"unclosed block",
			"---\nname: test\nno closing\n",
			"---\nname: test\nno closing\n",
		},
		{
			"complex YAML in frontmatter",
			"---\nname: test\nitems:\n  - one\n  - two\nnested:\n  key: val\n---\n\n# Body\n",
			"# Body\n",
		},
		{
			"closing delimiter at EOF",
			"---\nname: test\ndescription: foo\n---",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripFrontmatter(tt.in)
			if got != tt.want {
				t.Errorf("StripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}
