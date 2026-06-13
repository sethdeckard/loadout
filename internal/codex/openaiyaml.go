// Package codex handles the Codex agent policy file, agents/openai.yaml. It
// merges Loadout's resolved invocation policy into an existing (possibly
// author-written) file while preserving every other field, and reads policy
// back out for import inference.
package codex

import (
	"fmt"

	yaml "go.yaml.in/yaml/v3"

	"github.com/sethdeckard/loadout/internal/domain"
)

// OpenAIYAMLPath is the skill-relative location of the Codex agent policy file.
const OpenAIYAMLPath = "agents/openai.yaml"

// Merge sets policy.allow_implicit_invocation to allowImplicit within the YAML
// document in existing, preserving all other fields and nested values. A nil or
// empty existing produces a minimal document containing only the policy block.
// Comments and key ordering are not preserved.
func Merge(existing []byte, allowImplicit bool) ([]byte, error) {
	doc := map[string]any{}
	if len(existing) > 0 {
		if err := yaml.Unmarshal(existing, &doc); err != nil {
			return nil, fmt.Errorf("parse openai.yaml: %w", err)
		}
		if doc == nil {
			doc = map[string]any{}
		}
	}

	policy, ok := doc[domain.PolicyKey].(map[string]any)
	if !ok {
		policy = map[string]any{}
		doc[domain.PolicyKey] = policy
	}
	policy[domain.AllowImplicitInvocationKey] = allowImplicit

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal openai.yaml: %w", err)
	}
	return out, nil
}

// StripManagedPolicy removes the Loadout-managed policy.allow_implicit_invocation
// field (and the policy map if it becomes empty) and re-serializes the rest. It
// reports whether any content remains, so callers comparing skill trees can
// ignore the derived policy value while still distinguishing author-written
// fields (interface, dependencies, other policy keys). Unparseable input is
// returned verbatim so genuine differences still register.
func StripManagedPolicy(data []byte) (rest []byte, hasContent bool, err error) {
	if len(data) == 0 {
		return nil, false, nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return data, true, nil
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if policy, ok := doc[domain.PolicyKey].(map[string]any); ok {
		delete(policy, domain.AllowImplicitInvocationKey)
		if len(policy) == 0 {
			delete(doc, domain.PolicyKey)
		}
	}
	if len(doc) == 0 {
		return nil, false, nil
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, false, fmt.Errorf("marshal openai.yaml: %w", err)
	}
	return out, true, nil
}

// InferAllowImplicit reads policy.allow_implicit_invocation from an
// agents/openai.yaml document. present is false when the file is empty, fails to
// parse, or does not declare the policy.
func InferAllowImplicit(data []byte) (val bool, present bool) {
	if len(data) == 0 {
		return false, false
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, false
	}
	policy, ok := doc[domain.PolicyKey].(map[string]any)
	if !ok {
		return false, false
	}
	v, ok := policy[domain.AllowImplicitInvocationKey].(bool)
	if !ok {
		return false, false
	}
	return v, true
}
