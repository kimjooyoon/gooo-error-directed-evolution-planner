package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	ContractSchema       = "gooo/error_directed_evolution_planner/v1"
	TerminalSchema       = "gooo/reflexive-terminal-record/v1"
	CounterexampleSchema = "gooo/semantic-counterexample/v1"
	BundleSchema         = "gooo/error-directed-evolution-planner/candidate-bundle/v1"
	RationaleSchema      = "gooo/error-directed-evolution-planner/causal-rationale/v1"
	RollbackSchema       = "gooo/error-directed-evolution-planner/rollback-receipt/v1"
	ReportSchema         = "gooo/error-directed-evolution-planner/report/v1"
	ConformanceSchema    = "gooo/error-directed-evolution-planner/conformance/v1"
	DecisionClosed       = "CLOSED"
	DecisionUnknown      = "UNKNOWN"
	DecisionRefuted      = "REFUTED"
)

var requiredUnknownFields = []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}
var allowedOperations = []string{"ADD", "RETIRE", "SPLIT", "REWIRE"}

type Contract struct {
	Schema         string
	Authority      string
	Denominator    DenominatorDecl
	Statuses       []string
	Precedence     []string
	UnknownFields  []string
	SourcePolicy   map[string]string
	Toolchain      ToolchainDecl
	Operations     []OperationSpec
	CandidateOrder []string
	GenerationPlan GenerationPlan
	Scenarios      []ScenarioSpec
}

type DenominatorDecl struct {
	ID        string
	Scenarios int
	Unit      string
}

type ToolchainDecl struct {
	Go     string
	Digest string
}

type OperationSpec struct {
	Ordinal int
	Kind    string
	Target  string
	Cost    int
}

type GenerationPlan struct {
	Order   []string
	Outputs []string
}

type ScenarioSpec struct {
	Ordinal              int
	ID                   string
	Terminal             string
	Counterexample       string
	Guardrails           string
	Expected             string
	TerminalDigest       string
	CounterexampleDigest string
}

type TerminalRecord struct {
	Schema               string         `json:"schema"`
	Decision             string         `json:"decision"`
	Stage                string         `json:"stage"`
	Step                 string         `json:"step"`
	Reason               string         `json:"reason"`
	UnknownClass         string         `json:"unknown_class"`
	NextOperation        string         `json:"next_operation"`
	BlockedBy            []string       `json:"blocked_by"`
	CauseEdge            FrontierEdge   `json:"cause_edge"`
	MinimalFrontier      []FrontierEdge `json:"minimal_frontier"`
	Counterexample       string         `json:"counterexample"`
	CounterexampleDigest string         `json:"counterexample_digest"`
}

type FrontierEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	ValueType string `json:"value_type"`
}

type Counterexample struct {
	Schema              string         `json:"schema"`
	ID                  string         `json:"id"`
	Scenario            string         `json:"scenario"`
	SourceDigest        string         `json:"source_digest"`
	ToolchainDigest     string         `json:"toolchain_digest"`
	BaselinePhaseDigest string         `json:"baseline_phase_digest"`
	Decision            string         `json:"decision"`
	Reason              string         `json:"reason"`
	ReasonDigest        string         `json:"reason_digest"`
	MinimizedGraph      MinimizedGraph `json:"minimized_graph"`
	Repair              RepairSpec     `json:"repair"`
	Provenance          Provenance     `json:"provenance"`
}

type MinimizedGraph struct {
	Schema string      `json:"schema"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Payload string `json:"payload,omitempty"`
}

type GraphEdge struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`
	Payload string `json:"payload,omitempty"`
}

type RepairSpec struct {
	AcceptedOperations         []string `json:"accepted_operations"`
	ExpectedDecision           string   `json:"expected_decision"`
	ExpectedReason             string   `json:"expected_reason"`
	ExpectedActivityCount      int      `json:"expected_activity_count"`
	ExpectedLocalizationStages int      `json:"expected_localization_stages"`
	ReasonMustMatch            bool     `json:"reason_must_match"`
}

type Provenance struct {
	CompilerRepository string `json:"compiler_repository"`
	CompilerRelease    string `json:"compiler_release"`
	ReducerRepository  string `json:"reducer_repository"`
	ReducerRelease     string `json:"reducer_release"`
	AssetDigest        string `json:"asset_digest"`
}

type GuardrailCorpus struct {
	Schema string          `json:"schema"`
	ID     string          `json:"id"`
	Cases  []GuardrailCase `json:"cases"`
}

type GuardrailCase struct {
	ID                string `json:"id"`
	BaselineDecision  string `json:"baseline_decision"`
	CandidateDecision string `json:"candidate_decision"`
	Preserve          bool   `json:"preserve"`
}

type OracleResult struct {
	Decision           string `json:"decision"`
	Reason             string `json:"reason"`
	ReasonDigest       string `json:"reason_digest"`
	Valid              bool   `json:"valid"`
	ActivityCount      int    `json:"activity_count"`
	LocalizationStages int    `json:"localization_stages"`
	Invocations        int    `json:"invocations"`
	Error              string `json:"error,omitempty"`
}

type DeltaChange struct {
	Operation string `json:"operation"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	ValueType string `json:"value_type,omitempty"`
}

type CandidateOption struct {
	ID               string        `json:"id"`
	Kind             string        `json:"kind"`
	Target           string        `json:"target"`
	Cost             int           `json:"cost"`
	Rank             int           `json:"rank"`
	PhaseDigest      string        `json:"phase_digest"`
	PhasePath        string        `json:"phase_path"`
	Changes          []DeltaChange `json:"changes"`
	Oracle           OracleResult  `json:"oracle"`
	Decision         string        `json:"decision"`
	Reason           string        `json:"reason"`
	ReasonDigest     string        `json:"reason_digest"`
	EvidenceAccepted bool          `json:"evidence_accepted"`
	GuardrailsPassed bool          `json:"guardrails_passed"`
}

type CandidateBundle struct {
	Schema               string            `json:"schema"`
	Scenario             string            `json:"scenario"`
	Decision             string            `json:"decision"`
	SourceDigest         string            `json:"source_digest"`
	ContractDigest       string            `json:"contract_digest"`
	ToolchainDigest      string            `json:"toolchain_digest"`
	TerminalDigest       string            `json:"terminal_digest"`
	CounterexampleDigest string            `json:"counterexample_digest"`
	GuardrailDigest      string            `json:"guardrail_digest"`
	BaselinePhaseDigest  string            `json:"baseline_phase_digest"`
	CandidatePhaseDigest string            `json:"candidate_phase_digest,omitempty"`
	Selected             *CandidateOption  `json:"selected,omitempty"`
	Candidates           []CandidateOption `json:"candidates"`
	CausalFrontier       []FrontierEdge    `json:"causal_frontier"`
	Unknown              *UnknownRecord    `json:"unknown,omitempty"`
	Authority            AuthorityReceipt  `json:"authority"`
	Adoption             AdoptionBoundary  `json:"adoption"`
	SelfImprovement      SelfImprovement   `json:"self_improvement"`
	Improvement          Improvement       `json:"improvement"`
	BundleDigest         string            `json:"bundle_digest"`
	CandidatePhaseText   string            `json:"-"`
}

type AuthorityReceipt struct {
	RepositoryWrites       int  `json:"repository_writes"`
	InputRepositoryChanged bool `json:"input_repository_changed"`
	LocalTestExecutions    int  `json:"local_test_executions"`
	MergeAuthority         int  `json:"merge_authority"`
}

type AdoptionBoundary struct {
	CandidateOnly         bool `json:"candidate_only"`
	AppliedToInput        bool `json:"applied_to_input"`
	AutomaticMerge        bool `json:"automatic_merge"`
	SeparateAuthorityStep bool `json:"separate_authority_step"`
}

type SelfImprovement struct {
	Status                     string         `json:"status"`
	SameCounterexampleResolved bool           `json:"same_counterexample_resolved"`
	GuardrailCorpusPassed      bool           `json:"guardrail_corpus_passed"`
	GuardrailCases             int            `json:"guardrail_cases"`
	GuardrailPassed            int            `json:"guardrail_passed"`
	GuardrailRegressed         int            `json:"guardrail_regressed"`
	Unknown                    *UnknownRecord `json:"unknown,omitempty"`
}

type Improvement struct {
	Status          string         `json:"status"`
	Scenario        string         `json:"scenario"`
	SourceDigest    string         `json:"source_digest"`
	ContractDigest  string         `json:"contract_digest"`
	ToolchainDigest string         `json:"toolchain_digest"`
	MatchedPair     bool           `json:"matched_pair"`
	BeforeBytes     int            `json:"before_bytes"`
	AfterBytes      int            `json:"after_bytes"`
	Unknown         *UnknownRecord `json:"unknown,omitempty"`
}

type UnknownRecord struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type CausalRationale struct {
	Schema                string            `json:"schema"`
	Scenario              string            `json:"scenario"`
	Decision              string            `json:"decision"`
	Terminal              TerminalRecord    `json:"terminal"`
	CounterexampleID      string            `json:"counterexample_id"`
	BaselineReason        string            `json:"baseline_reason"`
	SelectedOperation     string            `json:"selected_operation"`
	CauseFrontier         []FrontierEdge    `json:"cause_frontier"`
	ReasonMustMatch       bool              `json:"reason_must_match"`
	ReasonMatched         bool              `json:"reason_matched"`
	CandidateReason       string            `json:"candidate_reason"`
	CandidateReasonDigest string            `json:"candidate_reason_digest"`
	Alternatives          []CandidateOption `json:"alternatives"`
	Unknown               *UnknownRecord    `json:"unknown,omitempty"`
	RationaleDigest       string            `json:"rationale_digest"`
}

type RollbackReceipt struct {
	Schema               string `json:"schema"`
	Scenario             string `json:"scenario"`
	Decision             string `json:"decision"`
	BaselinePhaseDigest  string `json:"baseline_phase_digest"`
	CandidatePhaseDigest string `json:"candidate_phase_digest"`
	InverseOperation     string `json:"inverse_operation"`
	ExactPair            bool   `json:"exact_pair"`
	AppliedToInput       bool   `json:"applied_to_input"`
	RepositoryWrites     int    `json:"repository_writes"`
	ReceiptDigest        string `json:"receipt_digest"`
}

type PlanReport struct {
	Schema          string          `json:"schema"`
	Scenario        string          `json:"scenario"`
	Decision        string          `json:"decision"`
	Bundle          CandidateBundle `json:"bundle"`
	Rationale       CausalRationale `json:"rationale"`
	Rollback        RollbackReceipt `json:"rollback"`
	SelfImprovement SelfImprovement `json:"self_improvement"`
}

type ConformanceReport struct {
	Schema    string       `json:"schema"`
	Decision  string       `json:"decision"`
	Scenarios int          `json:"scenarios"`
	Closed    int          `json:"closed"`
	Unknown   int          `json:"unknown"`
	Refuted   int          `json:"refuted"`
	Failed    int          `json:"failed"`
	Results   []PlanReport `json:"results"`
}

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestString(value string) string { return Digest([]byte(value)) }

func DigestJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal deterministic JSON: %v", err))
	}
	return Digest(data)
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func statusRank(status string) int {
	switch status {
	case DecisionRefuted:
		return 3
	case DecisionUnknown:
		return 2
	case DecisionClosed:
		return 1
	default:
		return 4
	}
}

func reduceStatus(statuses ...string) string {
	result := DecisionClosed
	for _, status := range statuses {
		if statusRank(status) > statusRank(result) {
			result = status
		}
	}
	return result
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func unknown(stage, step, reason, class, next string, blocked ...string) *UnknownRecord {
	return &UnknownRecord{Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: append([]string(nil), blocked...)}
}

func (u *UnknownRecord) valid() bool {
	return u != nil && u.Stage != "" && u.Step != "" && u.Reason != "" && u.UnknownClass != "" && u.NextOperation != "" && len(u.BlockedBy) > 0
}

func (r TerminalRecord) unknown() *UnknownRecord {
	return unknown(r.Stage, r.Step, r.Reason, r.UnknownClass, r.NextOperation, r.BlockedBy...)
}

func (c Counterexample) Validate() error {
	if c.Schema != CounterexampleSchema || c.ID == "" || c.SourceDigest == "" || c.ToolchainDigest == "" || !validDigest(c.SourceDigest) || !validDigest(c.ToolchainDigest) {
		return errors.New("counterexample identity is incomplete")
	}
	if c.Decision != DecisionRefuted || c.Reason == "" || !validDigest(c.ReasonDigest) {
		return errors.New("counterexample baseline decision or reason is incomplete")
	}
	if c.MinimizedGraph.Schema == "" || len(c.MinimizedGraph.Nodes) == 0 {
		return errors.New("minimized counterexample graph is empty")
	}
	seen := map[string]bool{}
	for _, node := range c.MinimizedGraph.Nodes {
		if node.ID == "" || seen[node.ID] {
			return fmt.Errorf("invalid minimized node %q", node.ID)
		}
		seen[node.ID] = true
	}
	for _, edge := range c.MinimizedGraph.Edges {
		if edge.ID == "" || edge.From == "" || edge.To == "" || !seen[edge.From] || !seen[edge.To] {
			return fmt.Errorf("invalid minimized edge %q", edge.ID)
		}
	}
	for _, operation := range c.Repair.AcceptedOperations {
		if !contains(allowedOperations, operation) {
			return fmt.Errorf("unsupported accepted operation %q", operation)
		}
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func equalStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}
