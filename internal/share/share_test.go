package share

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/importer"
)

type repoFixture struct {
	repoPath string
	skill    domain.Skill
}

func writeRepoSkill(t *testing.T, name string, targets []domain.Target, claudeMeta, codexMeta map[string]any, extras map[string][]byte) repoFixture {
	t.Helper()
	repo := t.TempDir()
	skillDir := filepath.Join(repo, "skills", name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\n\nBody for "+name+".\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	skill := domain.Skill{
		Name:        domain.SkillName(name),
		Description: "Description of " + name + ".",
		Targets:     targets,
		Claude:      claudeMeta,
		Codex:       codexMeta,
		Path:        filepath.Join("skills", name),
	}

	jsonBytes := []byte(`{"name":"` + name + `","description":"Description of ` + name + `.","targets":["` + targetsJSON(targets) + `"]}`)
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), jsonBytes, 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}

	for rel, data := range extras {
		full := filepath.Join(skillDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	return repoFixture{repoPath: repo, skill: skill}
}

func targetsJSON(targets []domain.Target) string {
	parts := make([]string, len(targets))
	for i, t := range targets {
		parts[i] = string(t)
	}
	return strings.Join(parts, `","`)
}

type tarEntry struct {
	name string
	mode int64
	data []byte
	dir  bool
}

func extractTarGz(t *testing.T, archivePath string) []tarEntry {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var entries []tarEntry
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		entry := tarEntry{name: header.Name, mode: header.Mode, dir: header.Typeflag == tar.TypeDir}
		if !entry.dir {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read entry %q: %v", header.Name, err)
			}
			entry.data = data
		}
		entries = append(entries, entry)
	}
	return entries
}

func entryNames(entries []tarEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

func findEntry(entries []tarEntry, name string) *tarEntry {
	for i := range entries {
		if entries[i].name == name {
			return &entries[i]
		}
	}
	return nil
}

func TestBuild_BothTargets(t *testing.T) {
	fix := writeRepoSkill(t,
		"both-targets",
		[]domain.Target{domain.TargetClaude, domain.TargetCodex},
		map[string]any{"allowed-tools": "Read, Grep"},
		map[string]any{},
		nil,
	)

	out := filepath.Join(t.TempDir(), "both-targets.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	entries := extractTarGz(t, out)
	names := entryNames(entries)

	mustHave := []string{
		"README.md",
		"claude-build/SKILL.md",
		"claude-build/skill.json",
		"codex-build/SKILL.md",
		"codex-build/skill.json",
		"loadout-source/SKILL.md",
		"loadout-source/skill.json",
	}
	for _, want := range mustHave {
		if findEntry(entries, want) == nil {
			t.Errorf("archive missing %q; got %v", want, names)
		}
	}

	claudeMD := findEntry(entries, "claude-build/SKILL.md")
	if claudeMD == nil || !bytes.Contains(claudeMD.data, []byte(`allowed-tools: "Read, Grep"`)) {
		t.Errorf("claude SKILL.md missing claude metadata; got %s", claudeMD.data)
	}

	codexMD := findEntry(entries, "codex-build/SKILL.md")
	if codexMD != nil && bytes.Contains(codexMD.data, []byte("allowed-tools")) {
		t.Errorf("codex SKILL.md should not have claude metadata; got %s", codexMD.data)
	}

	loadoutMD := findEntry(entries, "loadout-source/SKILL.md")
	if loadoutMD == nil || bytes.HasPrefix(loadoutMD.data, []byte("---\n")) {
		t.Errorf("loadout-source SKILL.md should be body-only (no frontmatter); got %s", loadoutMD.data)
	}
}

func TestBuild_ClaudeOnly(t *testing.T) {
	fix := writeRepoSkill(t,
		"claude-only",
		[]domain.Target{domain.TargetClaude},
		nil, nil, nil,
	)

	out := filepath.Join(t.TempDir(), "claude-only.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	entries := extractTarGz(t, out)
	for _, e := range entries {
		if strings.HasPrefix(e.name, "codex-build/") {
			t.Errorf("claude-only archive contains %q", e.name)
		}
	}
	if findEntry(entries, "claude-build/SKILL.md") == nil {
		t.Error("claude-only archive missing claude-build/SKILL.md")
	}
	readme := findEntry(entries, "README.md")
	if readme == nil {
		t.Fatal("missing README.md")
	}
	if bytes.Contains(readme.data, []byte("### For Codex")) {
		t.Errorf("claude-only README should not mention Codex; got %s", readme.data)
	}
	if !bytes.Contains(readme.data, []byte("### For Claude")) {
		t.Errorf("claude-only README should mention Claude; got %s", readme.data)
	}
}

func TestBuild_CodexOnly(t *testing.T) {
	fix := writeRepoSkill(t,
		"codex-only",
		[]domain.Target{domain.TargetCodex},
		nil, nil, nil,
	)

	out := filepath.Join(t.TempDir(), "codex-only.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	entries := extractTarGz(t, out)
	for _, e := range entries {
		if strings.HasPrefix(e.name, "claude-build/") {
			t.Errorf("codex-only archive contains %q", e.name)
		}
	}
	readme := findEntry(entries, "README.md")
	if readme == nil {
		t.Fatal("missing README.md")
	}
	if bytes.Contains(readme.data, []byte("### For Claude")) {
		t.Errorf("codex-only README should not mention Claude; got %s", readme.data)
	}
}

func TestBuild_FiltersJunk(t *testing.T) {
	fix := writeRepoSkill(t,
		"junk-skill",
		[]domain.Target{domain.TargetClaude},
		nil, nil,
		map[string][]byte{
			".loadout":             []byte(`{"repo_commit":"abc"}`),
			".DS_Store":            []byte("mac noise"),
			"references/.DS_Store": []byte("mac noise"),
			".git/HEAD":            []byte("ref: refs/heads/main"),
			"Thumbs.db":            []byte("windows noise"),
			"references/notes.md":  []byte("# Notes\n"),
		},
	)

	out := filepath.Join(t.TempDir(), "junk-skill.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	entries := extractTarGz(t, out)
	for _, e := range entries {
		base := filepath.Base(e.name)
		if base == ".loadout" || base == ".DS_Store" || base == "Thumbs.db" {
			t.Errorf("archive contains junk %q", e.name)
		}
		if strings.Contains(e.name, "/.git/") || strings.HasPrefix(e.name, ".git/") {
			t.Errorf("archive contains .git path %q", e.name)
		}
	}

	if findEntry(entries, "loadout-source/references/notes.md") == nil {
		t.Errorf("real extra file references/notes.md missing; got %v", entryNames(entries))
	}
}

func TestBuild_OutputCollision(t *testing.T) {
	fix := writeRepoSkill(t,
		"collide",
		[]domain.Target{domain.TargetClaude},
		nil, nil, nil,
	)

	out := filepath.Join(t.TempDir(), "collide.tar.gz")
	original := []byte("PRECIOUS")
	if err := os.WriteFile(out, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := Build(fix.repoPath, fix.skill, out)
	if err == nil {
		t.Fatal("Build over existing file: want error, got nil")
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("existing file modified; got %q want %q", got, original)
	}

	// No leftover temp file in the parent dir.
	parent := filepath.Dir(out)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "collide.tar.gz" {
			t.Errorf("stray file in output dir: %q", e.Name())
		}
	}
}

func TestBuild_MissingParent(t *testing.T) {
	fix := writeRepoSkill(t,
		"orphan-parent",
		[]domain.Target{domain.TargetClaude},
		nil, nil, nil,
	)

	out := filepath.Join(t.TempDir(), "missing", "subdir", "orphan-parent.tar.gz")
	err := Build(fix.repoPath, fix.skill, out)
	if err == nil {
		t.Fatal("Build with missing parent: want error, got nil")
	}
}

func TestBuild_Deterministic(t *testing.T) {
	fix := writeRepoSkill(t,
		"deterministic",
		[]domain.Target{domain.TargetClaude, domain.TargetCodex},
		map[string]any{"allowed-tools": "Read"},
		map[string]any{},
		map[string][]byte{
			"references/notes.md": []byte("# Notes\n"),
		},
	)

	dir := t.TempDir()
	first := filepath.Join(dir, "first.tar.gz")
	if err := Build(fix.repoPath, fix.skill, first); err != nil {
		t.Fatalf("Build first: %v", err)
	}

	time.Sleep(1100 * time.Millisecond) // ensure mtime would differ if leaked

	second := filepath.Join(dir, "second.tar.gz")
	if err := Build(fix.repoPath, fix.skill, second); err != nil {
		t.Fatalf("Build second: %v", err)
	}

	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("archives differ: len(first)=%d len(second)=%d", len(a), len(b))
	}
}

func TestBuild_PreservesExecutableBit(t *testing.T) {
	fix := writeRepoSkill(t,
		"exec-skill",
		[]domain.Target{domain.TargetClaude},
		nil, nil, nil,
	)
	scriptPath := filepath.Join(fix.repoPath, "skills", "exec-skill", "scripts", "run.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	out := filepath.Join(t.TempDir(), "exec-skill.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	entries := extractTarGz(t, out)
	for _, name := range []string{"loadout-source/scripts/run.sh", "claude-build/scripts/run.sh"} {
		e := findEntry(entries, name)
		if e == nil {
			t.Errorf("missing %q", name)
			continue
		}
		if e.mode&0o100 == 0 {
			t.Errorf("%s mode = %o, want owner-executable", name, e.mode)
		}
	}
}

func TestBuild_LoadoutSourceRoundTrip(t *testing.T) {
	fix := writeRepoSkill(t,
		"roundtrip",
		[]domain.Target{domain.TargetClaude, domain.TargetCodex},
		map[string]any{"allowed-tools": "Read"},
		map[string]any{},
		map[string][]byte{
			"references/notes.md": []byte("# Notes\n"),
		},
	)

	out := filepath.Join(t.TempDir(), "roundtrip.tar.gz")
	if err := Build(fix.repoPath, fix.skill, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	extractDir := t.TempDir()
	extractToDisk(t, out, extractDir)

	loadoutSrc := filepath.Join(extractDir, "loadout-source")
	freshRepo := t.TempDir()

	result, err := importer.Import(importer.ImportParams{
		SourceDir: loadoutSrc,
		RepoPath:  freshRepo,
	})
	if err != nil {
		t.Fatalf("importer.Import: %v", err)
	}
	if result.Skill.Name != "roundtrip" {
		t.Errorf("imported name = %q, want %q", result.Skill.Name, "roundtrip")
	}

	imported := filepath.Join(freshRepo, "skills", "roundtrip")
	original := filepath.Join(fix.repoPath, "skills", "roundtrip")
	assertSameTree(t, original, imported, map[string]bool{".loadout": true, "skill.json": true})

	// skill.json compared structurally (importer reformats but preserves content)
	var origSkill, importedSkill domain.Skill
	mustReadJSON(t, filepath.Join(original, "skill.json"), &origSkill)
	mustReadJSON(t, filepath.Join(imported, "skill.json"), &importedSkill)
	if origSkill.Name != importedSkill.Name {
		t.Errorf("Name: got %q, want %q", importedSkill.Name, origSkill.Name)
	}
	if origSkill.Description != importedSkill.Description {
		t.Errorf("Description: got %q, want %q", importedSkill.Description, origSkill.Description)
	}
	if !equalTargets(origSkill.Targets, importedSkill.Targets) {
		t.Errorf("Targets: got %v, want %v", importedSkill.Targets, origSkill.Targets)
	}
}

func mustReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

func equalTargets(a, b []domain.Target) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[domain.Target]int)
	for _, t := range a {
		seen[t]++
	}
	for _, t := range b {
		seen[t]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func extractToDisk(t *testing.T, archivePath, destDir string) {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		path := filepath.Join(destDir, filepath.FromSlash(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", path, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir parent %s: %v", path, err)
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				t.Fatalf("create %s: %v", path, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				t.Fatalf("copy %s: %v", path, err)
			}
			if err := out.Close(); err != nil {
				t.Fatalf("close %s: %v", path, err)
			}
		}
	}
}

func assertSameTree(t *testing.T, a, b string, ignore map[string]bool) {
	t.Helper()
	listA := walkRel(t, a, ignore)
	listB := walkRel(t, b, ignore)
	sort.Strings(listA)
	sort.Strings(listB)
	if !equalSlices(listA, listB) {
		t.Errorf("file sets differ:\n a=%v\n b=%v", listA, listB)
		return
	}
	for _, rel := range listA {
		da, _ := os.ReadFile(filepath.Join(a, rel))
		db, _ := os.ReadFile(filepath.Join(b, rel))
		if !bytes.Equal(da, db) {
			t.Errorf("%s differs:\n a=%q\n b=%q", rel, da, db)
		}
	}
}

func walkRel(t *testing.T, root string, ignore map[string]bool) []string {
	t.Helper()
	var rels []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if ignore[filepath.Base(rel)] {
			return nil
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	return rels
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
