package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

var trustedClawHubHosts = map[string]struct{}{
	"clawhub.ai":     {},
	"www.clawhub.ai": {},
}

type InstallCheckReport struct {
	Version          int               `json:"version"`
	CheckedAt        string            `json:"checked_at"`
	Registry         string            `json:"registry"`
	Slug             string            `json:"slug"`
	InstalledVersion string            `json:"installed_version"`
	TrustedRegistry  bool              `json:"trusted_registry"`
	HasSkillMD       bool              `json:"has_skill_md"`
	FileCount        int               `json:"file_count"`
	ExecutableFiles  []string          `json:"executable_files,omitempty"`
	BinaryFiles      []string          `json:"binary_files,omitempty"`
	Symlinks         []string          `json:"symlinks,omitempty"`
	SHA256           map[string]string `json:"sha256"`
	Passed           bool              `json:"passed"`
	FailureReasons   []string          `json:"failure_reasons,omitempty"`
}

func ValidateTrustedClawHubConfig(cfg ClawHubConfig) error {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://clawhub.ai"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("registry must use https")
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := trustedClawHubHosts[host]; !ok {
		return fmt.Errorf("registry host %q is not trusted", host)
	}
	return nil
}

func VerifyInstalledSkill(targetDir, registryName, slug, version string) (*InstallCheckReport, error) {
	report := &InstallCheckReport{
		Version:          1,
		CheckedAt:        time.Now().Format(time.RFC3339),
		Registry:         registryName,
		Slug:             slug,
		InstalledVersion: version,
		TrustedRegistry:  registryName == "clawhub",
		SHA256:           map[string]string{},
	}

	err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(targetDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			report.Symlinks = append(report.Symlinks, filepath.ToSlash(rel))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		report.FileCount++
		rel = filepath.ToSlash(rel)
		if rel == "SKILL.md" {
			report.HasSkillMD = true
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		report.SHA256[rel] = hex.EncodeToString(sum[:])
		if info.Mode()&0o111 != 0 {
			report.ExecutableFiles = append(report.ExecutableFiles, rel)
		}
		sample := data
		if len(sample) > 4096 {
			sample = sample[:4096]
		}
		if strings.IndexByte(string(sample), 0) >= 0 {
			report.BinaryFiles = append(report.BinaryFiles, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(report.ExecutableFiles)
	sort.Strings(report.BinaryFiles)
	sort.Strings(report.Symlinks)

	if !report.TrustedRegistry {
		report.FailureReasons = append(report.FailureReasons, "registry is not trusted")
	}
	if !report.HasSkillMD {
		report.FailureReasons = append(report.FailureReasons, "missing top-level SKILL.md")
	}
	if len(report.Symlinks) > 0 {
		report.FailureReasons = append(report.FailureReasons, "skill contains symlinks")
	}
	if len(report.ExecutableFiles) > 0 {
		report.FailureReasons = append(report.FailureReasons, "skill contains executable files")
	}
	if len(report.BinaryFiles) > 0 {
		report.FailureReasons = append(report.FailureReasons, "skill contains binary files")
	}
	report.Passed = len(report.FailureReasons) == 0

	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if writeErr := fileutil.WriteFileAtomic(filepath.Join(targetDir, ".skill-check.json"), data, 0o600); writeErr != nil {
			return nil, writeErr
		}
	} else {
		return nil, err
	}

	return report, nil
}
