package domain

// Codex skill-invocation policy keys. The native Codex shape nests
// AllowImplicitInvocationKey under PolicyKey; DisableModelInvocationKey is the
// Claude-style compatibility alias. These keys are policy-only metadata and
// never appear in generated Codex SKILL.md frontmatter.
const (
	PolicyKey                  = "policy"
	AllowImplicitInvocationKey = "allow_implicit_invocation"
	DisableModelInvocationKey  = "disable-model-invocation"
)

// ResolveCodexPolicy derives the effective allow_implicit_invocation value from
// a skill's Codex metadata map. The native codex.policy.allow_implicit_invocation
// wins when present; otherwise the disable-model-invocation alias is inverted.
// present is false when neither key is declared, in which case Loadout does not
// synthesize any policy.
func ResolveCodexPolicy(codex map[string]any) (allowImplicit bool, present bool) {
	if policy, ok := codex[PolicyKey].(map[string]any); ok {
		if v, ok := policy[AllowImplicitInvocationKey].(bool); ok {
			return v, true
		}
	}
	if v, ok := codex[DisableModelInvocationKey].(bool); ok {
		return !v, true
	}
	return false, false
}
