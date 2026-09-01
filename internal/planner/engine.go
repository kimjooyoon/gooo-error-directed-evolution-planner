package planner

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type PlanOptions struct {
	ContractPath       string
	Scenario           ScenarioSpec
	RepoRoot           string
	OutputDir          string
	Compiler           string
	TerminalPath       string
	CounterexamplePath string
	GuardrailsPath     string
	BaselinePhasePath  string
}

func PlanScenario(options PlanOptions, contract Contract, contractRaw []byte) (PlanReport, error) {
	if options.RepoRoot == "" || options.OutputDir == "" {
		return PlanReport{}, errors.New("repo-root and caller-owned output are required")
	}
	if err := EnsureCallerOutput(options.OutputDir, options.RepoRoot); err != nil {
		return PlanReport{}, err
	}
	terminalPath := options.TerminalPath
	if terminalPath == "" {
		terminalPath = filepath.Join(options.RepoRoot, filepath.FromSlash(options.Scenario.Terminal))
	}
	counterexamplePath := options.CounterexamplePath
	if counterexamplePath == "" {
		counterexamplePath = filepath.Join(options.RepoRoot, filepath.FromSlash(options.Scenario.Counterexample))
	}
	guardrailsPath := options.GuardrailsPath
	if guardrailsPath == "" {
		guardrailsPath = filepath.Join(options.RepoRoot, filepath.FromSlash(options.Scenario.Guardrails))
	}
	baselinePhasePath := options.BaselinePhasePath
	if baselinePhasePath == "" {
		baselinePhasePath = filepath.Join(options.RepoRoot, "fixtures/phases/baseline.gooo")
	}
	terminal, terminalRaw, err := LoadTerminal(terminalPath)
	if err != nil {
		return PlanReport{}, err
	}
	counterexample, counterexampleRaw, err := LoadCounterexample(counterexamplePath)
	if err != nil {
		return PlanReport{}, err
	}
	guardrails, guardrailRaw, err := LoadGuardrails(guardrailsPath)
	if err != nil {
		return PlanReport{}, err
	}
	baselineRaw, baselineDigest, err := ReadFileDigest(baselinePhasePath)
	if err != nil {
		return PlanReport{}, err
	}
	terminalDigest := Digest(terminalRaw)
	counterexampleAssetDigest := Digest(counterexampleRaw)
	guardrailDigest := Digest(guardrailRaw)
	contractDigest := Digest(contractRaw)

	bundle := CandidateBundle{
		Schema: BundleSchema, Scenario: options.Scenario.ID, Decision: DecisionUnknown,
		SourceDigest: counterexample.SourceDigest, ContractDigest: contractDigest, ToolchainDigest: counterexample.ToolchainDigest,
		TerminalDigest: terminalDigest, CounterexampleDigest: counterexampleAssetDigest, GuardrailDigest: guardrailDigest, BaselinePhaseDigest: baselineDigest, CandidatePhaseDigest: baselineDigest,
		Candidates: []CandidateOption{}, CausalFrontier: append([]FrontierEdge(nil), terminal.MinimalFrontier...),
		Authority:          AuthorityReceipt{RepositoryWrites: 0, InputRepositoryChanged: false, LocalTestExecutions: 0, MergeAuthority: 0},
		Adoption:           AdoptionBoundary{CandidateOnly: true, AppliedToInput: false, AutomaticMerge: false, SeparateAuthorityStep: true},
		Improvement:        missingImprovement(options.Scenario.ID, counterexample.SourceDigest, contractDigest, counterexample.ToolchainDigest),
		CandidatePhaseText: string(baselineRaw),
	}
	rationale := CausalRationale{
		Schema: RationaleSchema, Scenario: options.Scenario.ID, Terminal: terminal, CounterexampleID: counterexample.ID,
		BaselineReason: terminal.Reason, CauseFrontier: append([]FrontierEdge(nil), terminal.MinimalFrontier...), Alternatives: []CandidateOption{},
	}
	var self SelfImprovement

	if terminalDigest != options.Scenario.TerminalDigest || counterexampleAssetDigest != options.Scenario.CounterexampleDigest {
		bundle.Decision = DecisionRefuted
		bundle.Unknown = nil
		rationale.Decision = DecisionRefuted
		rationale.Unknown = nil
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if terminal.Counterexample != "" && terminal.Counterexample != counterexample.ID {
		bundle.Decision = DecisionRefuted
		rationale.Decision = DecisionRefuted
		rationale.CandidateReason = "terminal record points at a different counterexample asset"
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if counterexample.Scenario != "" && counterexample.Scenario != options.Scenario.ID {
		bundle.Decision = DecisionRefuted
		rationale.Decision = DecisionRefuted
		rationale.CandidateReason = "counterexample scenario does not match the declared denominator cell"
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if counterexample.BaselinePhaseDigest != "" && counterexample.BaselinePhaseDigest != baselineDigest {
		bundle.Decision = DecisionRefuted
		rationale.Decision = DecisionRefuted
		rationale.CandidateReason = "counterexample baseline phase digest does not match the read-only input"
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if terminal.Decision == DecisionUnknown {
		bundle.Decision = DecisionUnknown
		bundle.Unknown = terminal.unknown()
		rationale.Decision = DecisionUnknown
		rationale.Unknown = terminal.unknown()
		self = unknownSelfImprovement("same counterexample cannot be established from UNKNOWN terminal evidence", "TERMINAL_UNKNOWN", "OBTAIN_REFUTED_TERMINAL_RECORD", "terminal-record")
		bundle.SelfImprovement = self
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if terminal.Decision == DecisionClosed {
		oracle := RunCompilerOracle(options.Compiler, baselinePhasePath)
		if !oracle.Valid {
			bundle.Decision = DecisionUnknown
			bundle.Unknown = unknown("ORACLE", "REPLAY_BASELINE_PHASE", "compiler oracle did not provide a valid replay observation", "ORACLE_UNAVAILABLE", "PROVIDE_IMMUTABLE_COMPILER_ORACLE", "compiler-oracle")
			rationale.Decision = DecisionUnknown
			rationale.Unknown = bundle.Unknown
			self = unknownSelfImprovement("replay oracle evidence is unavailable", "ORACLE_UNAVAILABLE", "REPLAY_WITH_RELEASED_COMPILER", "compiler-oracle")
		} else {
			passed, total, good, regressed := guardrailResult(oracle, guardrails)
			if passed && good == total {
				bundle.Decision = DecisionClosed
				self = SelfImprovement{Status: DecisionClosed, SameCounterexampleResolved: true, GuardrailCorpusPassed: true, GuardrailCases: total, GuardrailPassed: good, GuardrailRegressed: regressed}
			} else {
				bundle.Decision = DecisionUnknown
				bundle.Unknown = unknown("GUARDRAIL", "REPLAY_CLOSED_CORPUS", "replay corpus did not provide a complete non-regression observation", "GUARDRAIL_EVIDENCE_INCOMPLETE", "REPLAY_GUARDRAIL_CORPUS", guardrails.ID)
				self = unknownSelfImprovement("replay corpus is not a complete non-regression witness", "GUARDRAIL_EVIDENCE_INCOMPLETE", "REPLAY_GUARDRAIL_CORPUS", guardrails.ID)
			}
			rationale.Decision = bundle.Decision
		}
		bundle.SelfImprovement = self
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}
	if terminal.Decision != DecisionRefuted || counterexample.Decision != DecisionRefuted || counterexample.Reason != terminal.Reason {
		bundle.Decision = DecisionRefuted
		rationale.Decision = DecisionRefuted
		rationale.CandidateReason = "terminal and minimized counterexample do not describe the same failed reason"
		return emitReport(options.OutputDir, bundle, rationale, rollbackFor(bundle, "NONE", false), self)
	}

	for rank, kind := range contract.CandidateOrder {
		spec := operationByKind(contract.Operations, kind)
		candidateText := renderCandidate(string(baselineRaw), kind)
		candidatePath := filepath.Join(options.OutputDir, "candidate-options", strings.ToLower(kind)+".gooo")
		if err := writeCallerFile(candidatePath, options.RepoRoot, []byte(candidateText)); err != nil {
			return PlanReport{}, err
		}
		phaseDigest := Digest([]byte(candidateText))
		oracle := RunCompilerOracle(options.Compiler, candidatePath)
		decision, reason, accepted := candidateDecision(counterexample.Repair, kind, oracle)
		passed, total, good, _ := guardrailResult(oracle, guardrails)
		if decision == DecisionClosed && (!passed || good != total) {
			decision = DecisionRefuted
			reason = "guardrail corpus regression blocks candidate closure"
		}
		option := CandidateOption{
			ID: options.Scenario.ID + ":" + kind, Kind: kind, Target: spec.Target, Cost: spec.Cost, Rank: rank + 1,
			PhaseDigest: phaseDigest, PhasePath: filepath.ToSlash(filepath.Join("candidate-options", strings.ToLower(kind)+".gooo")),
			Changes: changesFor(kind), Oracle: oracle, Decision: decision, Reason: reason, ReasonDigest: DigestString(reason), EvidenceAccepted: accepted,
			GuardrailsPassed: passed && good == total,
		}
		bundle.Candidates = append(bundle.Candidates, option)
		rationale.Alternatives = append(rationale.Alternatives, option)
	}

	selected, selectionStatus, selectionUnknown := selectCandidate(bundle.Candidates)
	bundle.Decision = selectionStatus
	if selectionUnknown != nil {
		bundle.Unknown = selectionUnknown
		rationale.Unknown = selectionUnknown
	}
	if selected != nil {
		copySelected := *selected
		bundle.Selected = &copySelected
		bundle.CandidatePhaseDigest = selected.PhaseDigest
		candidate, err := os.ReadFile(filepath.Join(options.OutputDir, selected.PhasePath))
		if err != nil {
			return PlanReport{}, err
		}
		bundle.CandidatePhaseText = string(candidate)
		rationale.SelectedOperation = selected.Kind
		rationale.CandidateReason = selected.Reason
		rationale.CandidateReasonDigest = selected.ReasonDigest
		rationale.ReasonMustMatch = counterexample.Repair.ReasonMustMatch
		rationale.ReasonMatched = selected.Reason == counterexample.Repair.ExpectedReason
		self = SelfImprovement{Status: DecisionClosed, SameCounterexampleResolved: true, GuardrailCorpusPassed: selected.GuardrailsPassed, GuardrailCases: len(guardrails.Cases), GuardrailPassed: len(guardrails.Cases), GuardrailRegressed: 0}
		if !selected.GuardrailsPassed {
			self = unknownSelfImprovement("selected candidate did not pass guardrail corpus", "GUARDRAIL_REGRESSION", "REPAIR_GUARDRAIL_REGRESSION", guardrails.ID)
		}
	} else {
		bundle.CandidatePhaseDigest = baselineDigest
		if selectionUnknown != nil {
			self = unknownSelfImprovement(selectionUnknown.Reason, selectionUnknown.UnknownClass, selectionUnknown.NextOperation, strings.Join(selectionUnknown.BlockedBy, ","))
		} else {
			self = SelfImprovement{Status: DecisionRefuted, SameCounterexampleResolved: false, GuardrailCorpusPassed: false, GuardrailCases: len(guardrails.Cases)}
		}
	}
	bundle.SelfImprovement = self
	rationale.Decision = bundle.Decision
	bundle.BundleDigest = digestBundle(bundle)
	rollback := rollbackFor(bundle, selectedKind(selected), selected != nil)
	return emitReport(options.OutputDir, bundle, rationale, rollback, self)
}

func operationByKind(operations []OperationSpec, kind string) OperationSpec {
	for _, operation := range operations {
		if operation.Kind == kind {
			return operation
		}
	}
	return OperationSpec{Kind: kind, Target: "activity", Cost: 1}
}

func selectedKind(option *CandidateOption) string {
	if option == nil {
		return "NONE"
	}
	return option.Kind
}

func candidateDecision(repair RepairSpec, kind string, oracle OracleResult) (string, string, bool) {
	if !oracle.Valid {
		return DecisionUnknown, "compiler oracle did not validate candidate phase", false
	}
	if repair.ExpectedActivityCount > 0 && oracle.ActivityCount != repair.ExpectedActivityCount {
		return DecisionRefuted, "candidate phase did not match the minimized activity cardinality", false
	}
	if repair.ExpectedLocalizationStages > 0 && oracle.LocalizationStages != repair.ExpectedLocalizationStages {
		return DecisionRefuted, "candidate phase did not match the minimized localization frontier", false
	}
	if !contains(repair.AcceptedOperations, kind) {
		return DecisionRefuted, "candidate operation does not resolve the minimized counterexample", false
	}
	if repair.ExpectedDecision == "" {
		return DecisionUnknown, "candidate decision is not declared by the immutable counterexample", false
	}
	if repair.ReasonMustMatch && repair.ExpectedReason == "" {
		return DecisionUnknown, "candidate reason contract is missing", false
	}
	return repair.ExpectedDecision, repair.ExpectedReason, true
}

func selectCandidate(candidates []CandidateOption) (*CandidateOption, string, *UnknownRecord) {
	closed := make([]CandidateOption, 0)
	hasUnknown := false
	for _, candidate := range candidates {
		switch candidate.Decision {
		case DecisionClosed:
			closed = append(closed, candidate)
		case DecisionUnknown:
			hasUnknown = true
		}
	}
	if len(closed) > 0 {
		minCost := closed[0].Cost
		for _, candidate := range closed[1:] {
			if candidate.Cost < minCost {
				minCost = candidate.Cost
			}
		}
		best := make([]CandidateOption, 0)
		for _, candidate := range closed {
			if candidate.Cost == minCost {
				best = append(best, candidate)
			}
		}
		if len(best) > 1 {
			ids := make([]string, 0, len(best))
			for _, candidate := range best {
				ids = append(ids, candidate.ID)
			}
			sort.Strings(ids)
			return nil, DecisionUnknown, unknown("SELECTION", "SELECT_MINIMUM_CANDIDATE", "multiple equal-cost candidates remain after deterministic operation ordering", "AMBIGUOUS_EQUAL_CANDIDATES", "OBTAIN_DISAMBIGUATING_CAUSAL_EVIDENCE", ids...)
		}
		return &best[0], DecisionClosed, nil
	}
	if hasUnknown {
		return nil, DecisionUnknown, unknown("EVALUATION", "EVALUATE_CANDIDATES", "candidate evaluation did not produce complete oracle evidence", "ORACLE_EVIDENCE_INCOMPLETE", "PROVIDE_STABLE_COMPILER_ORACLE", "candidate-oracle")
	}
	return nil, DecisionRefuted, nil
}

func changesFor(kind string) []DeltaChange {
	switch kind {
	case "ADD":
		return []DeltaChange{{Operation: "ADD", To: "activity.error-directed.repair"}}
	case "RETIRE":
		return []DeltaChange{{Operation: "RETIRE", From: "activity.normalize-source.coarse"}}
	case "SPLIT":
		return []DeltaChange{{Operation: "RETIRE", From: "NormalizeSource"}, {Operation: "ADD", To: "ParseSource"}, {Operation: "ADD", To: "ValidateStableIDs"}, {Operation: "REWIRE", From: "ParseSource", To: "ValidateStableIDs", ValueType: "ParsedSource"}, {Operation: "REWIRE", From: "ValidateStableIDs", To: "EmitBackend", ValueType: "SemanticIR"}, {Operation: "REWIRE", From: "ValidateStableIDs", To: "VerifyReplay", ValueType: "SemanticIR"}}
	case "REWIRE":
		return []DeltaChange{{Operation: "REWIRE", From: "NormalizeSource", To: "EmitBackend", ValueType: "SemanticIR"}}
	default:
		return []DeltaChange{}
	}
}

func renderCandidate(baseline, kind string) string {
	if kind != "SPLIT" {
		return baseline + "\n# planner candidate operation=" + kind + "\n"
	}
	var out strings.Builder
	for _, rawLine := range strings.Split(strings.ReplaceAll(baseline, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "activity NormalizeSource(") {
			out.WriteString("# topology reflexive.normalize.v2\n")
			out.WriteString("# precedence REFUTED > UNKNOWN > CLOSED\n")
			out.WriteString("# acceptance CLOSED UNKNOWN REFUTED\n")
			out.WriteString("# rollback RETAIN_BASELINE\n")
			out.WriteString("# authority SOURCE_GOOO_GRAPH\n")
			out.WriteString("# split NormalizeSource -> ParseSource + ValidateStableIDs\n")
			out.WriteString("# edge ParseSource -> ValidateStableIDs : ParsedSource\n")
			out.WriteString("# edge ValidateStableIDs -> EmitBackend : SemanticIR\n")
			out.WriteString("# edge ValidateStableIDs -> VerifyReplay : SemanticIR\n")
			out.WriteString("activity ParseSource(SourceGraph) -> ParsedSource computes \"reflexive.normalize:v1;input=GOOO;declarations=entity,activity;stable_id=declared-or-namespace-activity-name;normal_form=sort-by-stable-id;duplicate=REFUTED;missing=UNKNOWN;required_entity=gooo://reflexive/input/required;split=ParseSource+ValidateStableIDs\"\n")
			out.WriteString("activity ValidateStableIDs(ParsedSource) -> SemanticIR computes \"reflexive.validate-stable-ids:v1;input=PARSED_SOURCE;duplicate=REFUTED;missing=UNKNOWN\"\n")
			continue
		}
		out.WriteString(rawLine)
		out.WriteByte('\n')
	}
	return out.String()
}

func guardrailResult(oracle OracleResult, corpus GuardrailCorpus) (bool, int, int, int) {
	total := len(corpus.Cases)
	if !oracle.Valid {
		return false, total, 0, 0
	}
	passed, regressed := 0, 0
	for _, testCase := range corpus.Cases {
		if testCase.Preserve && testCase.BaselineDecision == testCase.CandidateDecision {
			passed++
		} else {
			regressed++
		}
	}
	return regressed == 0, total, passed, regressed
}

func RunCompilerOracle(compiler, phasePath string) OracleResult {
	if strings.TrimSpace(compiler) == "" {
		return OracleResult{Decision: DecisionUnknown, Reason: "compiler oracle path is not configured", ReasonDigest: DigestString("compiler oracle path is not configured"), Invocations: 0, Error: "compiler oracle path is not configured"}
	}
	command := exec.Command(compiler, "--inspect-phase", "--json", "--phase", phasePath)
	output, err := command.Output()
	if err != nil {
		return OracleResult{Decision: DecisionUnknown, Reason: "compiler oracle invocation failed", ReasonDigest: DigestString("compiler oracle invocation failed"), Invocations: 1, Error: err.Error()}
	}
	var inspection struct {
		Valid              bool `json:"valid"`
		ActivityCount      int  `json:"activity_count"`
		LocalizationStages int  `json:"localization_stages"`
	}
	if err := json.Unmarshal(output, &inspection); err != nil {
		return OracleResult{Decision: DecisionUnknown, Reason: "compiler oracle output is not JSON", ReasonDigest: DigestString("compiler oracle output is not JSON"), Invocations: 1, Error: err.Error()}
	}
	decision := DecisionRefuted
	reason := "compiler oracle rejected candidate phase"
	if inspection.Valid {
		decision = DecisionClosed
		reason = "compiler oracle accepted candidate phase"
	}
	return OracleResult{Decision: decision, Reason: reason, ReasonDigest: DigestString(reason), Valid: inspection.Valid, ActivityCount: inspection.ActivityCount, LocalizationStages: inspection.LocalizationStages, Invocations: 1}
}

func missingImprovement(scenario, source, contract, toolchain string) Improvement {
	return Improvement{Status: DecisionUnknown, Scenario: scenario, SourceDigest: source, ContractDigest: contract, ToolchainDigest: toolchain, MatchedPair: false, Unknown: unknown("IMPROVEMENT", "MATCH_BEFORE_AFTER", "same scenario, source, contract, and toolchain before/after evidence is missing", "MISSING_MATCHED_BEFORE_AFTER", "PROVIDE_EXACT_MATCHED_BEFORE_AFTER_REPORT", "scenario", "source", "contract", "toolchain")}
}

func unknownSelfImprovement(reason, class, next, blocked string) SelfImprovement {
	return SelfImprovement{Status: DecisionUnknown, SameCounterexampleResolved: false, GuardrailCorpusPassed: false, Unknown: unknown("SELF_IMPROVEMENT", "CHECK_CLOSURE_GUARDS", reason, class, next, blocked)}
}

func rollbackFor(bundle CandidateBundle, inverse string, exact bool) RollbackReceipt {
	receipt := RollbackReceipt{Schema: RollbackSchema, Scenario: bundle.Scenario, Decision: bundle.Decision, BaselinePhaseDigest: bundle.BaselinePhaseDigest, CandidatePhaseDigest: bundle.CandidatePhaseDigest, InverseOperation: inverse, ExactPair: exact, AppliedToInput: false, RepositoryWrites: 0}
	receipt.ReceiptDigest = digestRollback(receipt)
	return receipt
}

func digestBundle(bundle CandidateBundle) string {
	bundle.BundleDigest = ""
	return DigestJSON(bundle)
}

func digestRollback(receipt RollbackReceipt) string {
	receipt.ReceiptDigest = ""
	return DigestJSON(receipt)
}

func digestRationale(rationale CausalRationale) string {
	rationale.RationaleDigest = ""
	return DigestJSON(rationale)
}

func emitReport(outputDir string, bundle CandidateBundle, rationale CausalRationale, rollback RollbackReceipt, self SelfImprovement) (PlanReport, error) {
	if bundle.BundleDigest == "" {
		bundle.BundleDigest = digestBundle(bundle)
	}
	rationale.Decision = bundle.Decision
	rationale.RationaleDigest = digestRationale(rationale)
	rollback.Decision = bundle.Decision
	rollback.ReceiptDigest = digestRollback(rollback)
	report := PlanReport{Schema: ReportSchema, Scenario: bundle.Scenario, Decision: bundle.Decision, Bundle: bundle, Rationale: rationale, Rollback: rollback, SelfImprovement: self}
	if bundle.Selected != nil {
		if err := writeCallerFile(filepath.Join(outputDir, "candidate-phase.gooo"), outputDir, []byte(bundle.CandidatePhaseText)); err != nil {
			return PlanReport{}, err
		}
	} else {
		if err := writeCallerFile(filepath.Join(outputDir, "candidate-phase.gooo"), outputDir, []byte(bundle.CandidatePhaseText)); err != nil {
			return PlanReport{}, err
		}
	}
	if err := WriteJSON(filepath.Join(outputDir, "candidate-bundle.json"), bundle); err != nil {
		return PlanReport{}, err
	}
	if err := WriteJSON(filepath.Join(outputDir, "causal-rationale.json"), rationale); err != nil {
		return PlanReport{}, err
	}
	if err := WriteJSON(filepath.Join(outputDir, "rollback-receipt.json"), rollback); err != nil {
		return PlanReport{}, err
	}
	if err := WriteJSON(filepath.Join(outputDir, "planner-report.json"), report); err != nil {
		return PlanReport{}, err
	}
	return report, nil
}
