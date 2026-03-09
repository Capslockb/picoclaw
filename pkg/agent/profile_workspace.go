package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

var profilePathSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type profileMetadata struct {
	ProfileKey   string   `json:"profile_key"`
	BaseAgentID  string   `json:"base_agent_id"`
	Mode         string   `json:"mode"`
	CreatedAt    string   `json:"created_at"`
	Isolated     bool     `json:"isolated"`
	MutableFiles []string `json:"mutable_files"`
}

func sanitizeProfilePathPart(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return "anonymous"
	}
	safe := profilePathSanitizer.ReplaceAllString(trimmed, "_")
	safe = strings.Trim(safe, "._-")
	if safe == "" {
		return "anonymous"
	}
	return safe
}

func profileWorkspacePath(baseWorkspace, profileKey string) string {
	return filepath.Join(baseWorkspace, "profiles", sanitizeProfilePathPart(profileKey))
}

func ensureProfileWorkspace(baseWorkspace, profileWorkspace, baseAgentID, profileKey string) error {
	if err := os.MkdirAll(profileWorkspace, 0o755); err != nil {
		return err
	}
	for _, dir := range []string{"memory", "sessions", "state", "logs", "tmp", "exports"} {
		if err := os.MkdirAll(filepath.Join(profileWorkspace, dir), 0o755); err != nil {
			return err
		}
	}

	rootFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"IDENTITY.md",
		"HEARTBEAT.md",
		"README.txt",
		"defaults.sh",
		"wsl-codex-exec",
		"node-exec",
		"node-ssh",
	}
	for _, name := range rootFiles {
		if err := copyFileIfMissing(filepath.Join(baseWorkspace, name), filepath.Join(profileWorkspace, name)); err != nil {
			return err
		}
	}
	if err := copyDirIfMissing(filepath.Join(baseWorkspace, "skills"), filepath.Join(profileWorkspace, "skills")); err != nil {
		return err
	}

	profileFile := filepath.Join(profileWorkspace, "PROFILE.json")
	if _, err := os.Stat(profileFile); err == nil {
		return nil
	}
	meta := profileMetadata{
		ProfileKey:  profileKey,
		BaseAgentID: baseAgentID,
		Mode:        "isolated-user-profile",
		CreatedAt:   time.Now().Format(time.RFC3339),
		Isolated:    true,
		MutableFiles: []string{
			"PROFILE.json",
			"memory/MEMORY.md",
			"memory/YYYYMM/YYYYMMDD.md",
			"skills/*",
			"sessions/*",
			"state/*",
		},
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profileFile, append(data, '\n'), 0o644)
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func copyDirIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.Walk(src, func(path string, entryInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entryInfo.IsDir() {
			return os.MkdirAll(target, entryInfo.Mode().Perm())
		}
		return copyFileIfMissing(path, target)
	})
}

func cloneSubagentsConfig(src *config.SubagentsConfig) *config.SubagentsConfig {
	if src == nil {
		return nil
	}
	cloned := &config.SubagentsConfig{
		AllowAgents: slices.Clone(src.AllowAgents),
	}
	if src.Model != nil {
		cloned.Model = &config.AgentModelConfig{
			Primary:   src.Model.Primary,
			Fallbacks: slices.Clone(src.Model.Fallbacks),
		}
	}
	return cloned
}

func (r *AgentRegistry) GetOrCreateProfileAgent(baseAgentID, profileKey string) (*AgentInstance, bool, error) {
	baseID := routing.NormalizeAgentID(baseAgentID)
	normalizedProfile := strings.TrimSpace(strings.ToLower(profileKey))
	if normalizedProfile == "" {
		return nil, false, fmt.Errorf("profile key is required")
	}
	cacheKey := baseID + "|" + normalizedProfile

	r.mu.RLock()
	if agent, ok := r.profileAgents[cacheKey]; ok {
		r.mu.RUnlock()
		return agent, false, nil
	}
	baseAgent, ok := r.agents[baseID]
	r.mu.RUnlock()
	if !ok || baseAgent == nil {
		return nil, false, fmt.Errorf("base agent %s not found", baseID)
	}

	workspace := profileWorkspacePath(baseAgent.Workspace, normalizedProfile)
	if err := ensureProfileWorkspace(baseAgent.Workspace, workspace, baseAgent.ID, normalizedProfile); err != nil {
		return nil, false, err
	}

	agentCfg := &config.AgentConfig{
		ID:        baseAgent.ID,
		Name:      baseAgent.Name,
		Workspace: workspace,
		Skills:    slices.Clone(baseAgent.SkillsFilter),
		Subagents: cloneSubagentsConfig(baseAgent.Subagents),
		Model: &config.AgentModelConfig{
			Primary:   baseAgent.Model,
			Fallbacks: slices.Clone(baseAgent.Fallbacks),
		},
	}
	instance := NewAgentInstance(agentCfg, &r.cfg.Agents.Defaults, r.cfg, r.provider)

	r.mu.Lock()
	defer r.mu.Unlock()
	if agent, ok := r.profileAgents[cacheKey]; ok {
		return agent, false, nil
	}
	r.profileAgents[cacheKey] = instance
	return instance, true, nil
}
