package skillmd

import (
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func TestParse(t *testing.T) {
	content := "---\nname: Test Skill\ndescription: A: B\nallowed-tools: Read, Grep\n---\n\n# Test\nBody\n"
	parsed := Parse(content)

	if got, want := parsed.Fields["name"], "Test Skill"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := parsed.Fields["description"], "A: B"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
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
	if !strings.Contains(got, "description: A test skill.") {
		t.Fatalf("frontmatter missing description:\n%s", got)
	}
	if !strings.Contains(got, "allowed-tools: Read, Grep") {
		t.Fatalf("frontmatter missing target meta:\n%s", got)
	}
}
