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

func BuildFrontmatter(skill domain.Skill, target domain.Target) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("name: %s\n", skill.Name))
	b.WriteString(fmt.Sprintf("description: %s\n", skill.Description))

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
		b.WriteString(fmt.Sprintf("%s: %v\n", key, meta[key]))
	}
	return b.String()
}
