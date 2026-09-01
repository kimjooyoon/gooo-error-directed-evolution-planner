package planner

import (
	"fmt"
	"os"
	"path/filepath"
)

type RunnerManifest struct {
	Schema         string          `json:"schema"`
	Authority      string          `json:"authority"`
	ContractDigest string          `json:"contract_digest"`
	Denominator    DenominatorDecl `json:"denominator"`
	Statuses       []string        `json:"statuses"`
	Precedence     []string        `json:"precedence"`
	UnknownFields  []string        `json:"unknown_fields"`
	Operations     []OperationSpec `json:"operations"`
	CandidateOrder []string        `json:"candidate_order"`
	GenerationPlan GenerationPlan  `json:"generation_plan"`
	ScenarioIDs    []string        `json:"scenario_ids"`
	Toolchain      ToolchainDecl   `json:"toolchain"`
}

func Manifest(contract Contract, raw []byte) RunnerManifest {
	ids := make([]string, 0, len(contract.Scenarios))
	for _, scenario := range contract.Scenarios {
		ids = append(ids, scenario.ID)
	}
	return RunnerManifest{
		Schema: "gooo/error-directed-evolution-planner/runner-manifest/v1", Authority: contract.Authority, ContractDigest: Digest(raw),
		Denominator: contract.Denominator, Statuses: append([]string(nil), contract.Statuses...), Precedence: append([]string(nil), contract.Precedence...),
		UnknownFields: append([]string(nil), contract.UnknownFields...), Operations: append([]OperationSpec(nil), contract.Operations...), CandidateOrder: append([]string(nil), contract.CandidateOrder...),
		GenerationPlan: contract.GenerationPlan, ScenarioIDs: ids, Toolchain: contract.Toolchain,
	}
}

func RunConformance(contract Contract, raw []byte, repoRoot, outputDir, compiler string) (ConformanceReport, error) {
	if err := PrepareOutput(outputDir, repoRoot); err != nil {
		return ConformanceReport{}, err
	}
	if err := WriteJSON(filepath.Join(outputDir, "runner-manifest.json"), Manifest(contract, raw)); err != nil {
		return ConformanceReport{}, err
	}
	results := make([]PlanReport, 0, len(contract.Scenarios))
	report := ConformanceReport{Schema: ConformanceSchema, Decision: DecisionClosed, Scenarios: len(contract.Scenarios), Results: results}
	for _, scenario := range contract.Scenarios {
		scenarioDir := filepath.Join(outputDir, "scenarios", scenario.ID)
		if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
			return ConformanceReport{}, err
		}
		result, err := PlanScenario(PlanOptions{ContractPath: filepath.Join(repoRoot, ".gooo/evolution-planner.gooo"), Scenario: scenario, RepoRoot: repoRoot, OutputDir: scenarioDir, Compiler: compiler}, contract, raw)
		if err != nil {
			return ConformanceReport{}, fmt.Errorf("scenario %s: %w", scenario.ID, err)
		}
		report.Results = append(report.Results, result)
		switch result.Decision {
		case DecisionClosed:
			report.Closed++
		case DecisionUnknown:
			report.Unknown++
		case DecisionRefuted:
			report.Refuted++
		}
		if result.Decision != scenario.Expected {
			report.Failed++
		}
	}
	if report.Failed > 0 {
		report.Decision = DecisionRefuted
	} else {
		// The corpus decision is the verification result of the fixed denominator;
		// REFUTED is an expected scenario state, not a conformance failure.
		report.Decision = DecisionClosed
	}
	if err := WriteJSON(filepath.Join(outputDir, "conformance-report.json"), report); err != nil {
		return ConformanceReport{}, err
	}
	return report, nil
}

func reduceStatusFromReports(reports []PlanReport) string {
	status := DecisionClosed
	for _, report := range reports {
		status = reduceStatus(status, report.Decision)
	}
	return status
}
