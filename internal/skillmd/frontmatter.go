package skillmd

import (
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/sethdeckard/loadout/internal/domain"
)

type Parsed struct {
	Fields map[string]any
	Body   string
}

func Parse(content string) Parsed {
	inner, body, ok := splitFrontmatter(content)
	if !ok {
		return Parsed{Fields: map[string]any{}, Body: content}
	}
	fields := map[string]any{}
	if err := yaml.Unmarshal([]byte(inner), &fields); err != nil || fields == nil {
		fields = map[string]any{}
	}
	return Parsed{Fields: fields, Body: body}
}

func Strip(content string) string {
	return Parse(content).Body
}

func Heading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// StripFrontmatter removes the first leading ---...--- block by delimiter,
// without parsing the content between delimiters.
func StripFrontmatter(content string) string {
	_, body, ok := splitFrontmatter(content)
	if !ok {
		return content
	}
	return body
}

// splitFrontmatter separates a leading ---...--- block from the body using
// delimiter detection only. inner is the raw text between the delimiters; body
// is everything after the closing delimiter. ok is false when content has no
// well-formed frontmatter block, in which case the whole content is the body.
func splitFrontmatter(content string) (inner, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := content[4:]
	if idx := strings.Index(rest, "\n---\n"); idx != -1 {
		body = strings.TrimPrefix(rest[idx+5:], "\n")
		return rest[:idx], body, true
	}
	// Closing delimiter at EOF (no trailing newline).
	if strings.HasSuffix(rest, "\n---") {
		return rest[:len(rest)-4], "", true
	}
	return "", "", false
}

// BuildFrontmatter serializes a skill's identity and target-specific metadata as
// YAML frontmatter. name and description lead, followed by remaining metadata
// keys in sorted order. For the Codex target, policy-only keys are filtered out
// (the invocation policy lives in agents/openai.yaml, not the frontmatter).
func BuildFrontmatter(skill domain.Skill, target domain.Target) string {
	root := &yaml.Node{Kind: yaml.MappingNode}
	appendField(root, "name", string(skill.Name))
	appendField(root, "description", skill.Description)

	meta := skill.TargetMeta(target)
	keys := make([]string, 0, len(meta))
	for key := range meta {
		if target == domain.TargetCodex && isCodexPolicyKey(key) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		appendField(root, key, meta[key])
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return ""
	}
	return string(out)
}

func appendField(mapping *yaml.Node, key string, value any) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{}
	if err := valueNode.Encode(value); err != nil {
		return
	}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
}

func isCodexPolicyKey(key string) bool {
	return key == domain.PolicyKey || key == domain.DisableModelInvocationKey
}
