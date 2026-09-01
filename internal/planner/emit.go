package planner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func EnsureCallerOutput(outputDir, repoRoot string) error {
	if outputDir == "" || repoRoot == "" {
		return errors.New("caller-owned output and input repository root are required")
	}
	out, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return errors.New("caller-owned output must be outside the input repository")
	}
	if info, err := os.Stat(out); err == nil && !info.IsDir() {
		return errors.New("caller-owned output must be a directory")
	}
	return nil
}

func PrepareOutput(outputDir, repoRoot string) error {
	if err := EnsureCallerOutput(outputDir, repoRoot); err != nil {
		return err
	}
	info, err := os.Stat(outputDir)
	if os.IsNotExist(err) {
		return os.MkdirAll(outputDir, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("caller-owned output must be a directory")
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("caller-owned output directory must be empty")
	}
	return nil
}

func writeCallerFile(path, repoRoot string, data []byte) error {
	_ = repoRoot
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func LoadBeforeReport(path string) (Improvement, error) {
	if strings.TrimSpace(path) == "" {
		return Improvement{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Improvement{}, err
	}
	var report PlanReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return Improvement{}, fmt.Errorf("parse before report: %w", err)
	}
	return Improvement{Status: report.Decision, Scenario: report.Scenario, SourceDigest: report.Bundle.SourceDigest, ContractDigest: report.Bundle.ContractDigest, ToolchainDigest: report.Bundle.ToolchainDigest, BeforeBytes: len(raw), AfterBytes: len(raw), MatchedPair: true}, nil
}

func ApplyImprovement(bundle *CandidateBundle, before Improvement) {
	if bundle == nil || before.Scenario == "" {
		return
	}
	if before.Scenario != bundle.Scenario || before.SourceDigest != bundle.SourceDigest || before.ContractDigest != bundle.ContractDigest || before.ToolchainDigest != bundle.ToolchainDigest {
		bundle.Improvement = missingImprovement(bundle.Scenario, bundle.SourceDigest, bundle.ContractDigest, bundle.ToolchainDigest)
		bundle.Improvement.Unknown = unknown("IMPROVEMENT", "MATCH_BEFORE_AFTER", "before report does not share scenario, source, contract, and toolchain digests", "DIGEST_MISMATCH", "COLLECT_SAME_DIGEST_BEFORE_AFTER", "before-after-digest")
		return
	}
	bundle.Improvement = Improvement{Status: DecisionUnknown, Scenario: bundle.Scenario, SourceDigest: bundle.SourceDigest, ContractDigest: bundle.ContractDigest, ToolchainDigest: bundle.ToolchainDigest, MatchedPair: true, BeforeBytes: before.AfterBytes, AfterBytes: len(bundle.BundleDigest)}
	if before.AfterBytes > len(bundle.BundleDigest) {
		bundle.Improvement.Status = DecisionClosed
		bundle.Improvement.Unknown = nil
		return
	}
	bundle.Improvement.Status = DecisionRefuted
	bundle.Improvement.Unknown = nil
}
