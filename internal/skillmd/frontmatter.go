package skillmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sethdeckard/loadout/internal/domain"
)

type Parsed struct {
	Fields map[string]string
	Body   string
}

func Parse(content string) Parsed {
	if !strings.HasPrefix(content, "---\n") {
		return Parsed{Fields: map[string]string{}, Body: content}
	}

	lines := strings.Split(content, "\n")
	fields := make(map[string]string)
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
		line := lines[i]
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Parsed{Fields: map[string]string{}, Body: content}
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if end == -1 {
		return Parsed{Fields: map[string]string{}, Body: content}
	}

	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
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
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---\n")
	if end != -1 {
		body := content[4+end+5:] // skip past "\n---\n"
		body = strings.TrimPrefix(body, "\n")
		return body
	}
	// Handle closing --- at EOF (no trailing newline)
	if strings.HasSuffix(content, "\n---") {
		return ""
	}
	return content
}

// yamlQuote wraps s in double quotes, escaping internal backslashes and
// double-quote characters. This produces a valid YAML double-quoted scalar.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// formatValue formats a metadata value for YAML output. Strings are
// double-quoted to avoid YAML special-character issues. Bools and numbers
// are emitted unquoted to preserve their YAML type.
func formatValue(v any) string {
	switch v.(type) {
	case bool, int, int64, float64:
		return fmt.Sprint(v)
	default:
		return yamlQuote(fmt.Sprint(v))
	}
}

func BuildFrontmatter(skill domain.Skill, target domain.Target) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("name: %s\n", skill.Name))
	b.WriteString(fmt.Sprintf("description: %s\n", yamlQuote(skill.Description)))

	meta := skill.TargetMeta(target)
	if len(meta) == 0 {
		return b.String()
	}

	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("%s: %s\n", key, formatValue(meta[key])))
	}
	return b.String()
}
