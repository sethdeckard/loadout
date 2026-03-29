package app

import (
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/gitrepo"
	"github.com/sethdeckard/loadout/internal/reconcile"
)

type SkillView struct {
	Skill           domain.Skill
	InstalledClaude bool
	InstalledCodex  bool
	ManagedClaude   bool
	ManagedCodex    bool
	ProjectClaude   bool
	ProjectCodex    bool
	Flags           []reconcile.StatusFlag
	Orphaned        bool   // managed install present on disk but absent from repo
	OrphanRoot      string // target root where the orphaned install was found
	LocalRoot       string // local root for previewing unmanaged or orphaned skills
	LocalSourceDir  string // exact source directory for previewing importable project-local skills
}

type SyncResult struct {
	Pushed           bool
	Pulled           bool
	Bootstrapped     bool
	RepoChanged      bool
	RefreshedUser    int
	RefreshedProject int
	RefreshErrors    []string
}

func (r SyncResult) RefreshedTotal() int {
	return r.RefreshedUser + r.RefreshedProject
}

type SyncStatus struct {
	NeedsSync        bool
	RepoDirty        bool
	LocalAhead       bool
	RemoteAhead      bool
	Diverged         bool
	BootstrapNeeded  bool
	OutdatedUser     int
	OutdatedProject  int
	RepoState        gitrepo.SyncState
	RemoteCheckError error
}

type SkillPreview struct {
	Skill    domain.Skill
	Markdown string
	Files    []string // extra files beyond SKILL.md and skill.json
}

type ImportPreview struct {
	Skill     domain.Skill
	Markdown  string
	SourceDir string
	Ready     bool
	Problem   string
}

type DoctorCheck struct {
	Name   string
	OK     bool
	Detail string
}

type DoctorReport struct {
	Checks []DoctorCheck
	AllOK  bool
}

type ImportResult struct {
	SkillName     domain.SkillName
	DestDir       string
	RepoPath      string
	CommitCreated bool
}

type DeleteEligibility struct {
	SkillName   domain.SkillName
	DeletedPath string
	Deletable   bool
	Blockers    []string
}

type DeleteSkillResult struct {
	SkillName     domain.SkillName
	DeletedPath   string
	CommitCreated bool
}

type ImportCandidateView struct {
	SkillName domain.SkillName
	SourceDir string
	Targets   []domain.Target
	Ready     bool
	Problem   string
	Duplicate bool
	FromRoots []domain.Target
	Orphan    bool // source directory contains a .loadout marker (recovery candidate)
}
