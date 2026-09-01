package planner

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadContract(path string) (Contract, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, nil, err
	}
	contract, err := parseContract(string(raw))
	if err != nil {
		return Contract{}, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := contract.Validate(); err != nil {
		return Contract{}, nil, err
	}
	return contract, raw, nil
}

func parseContract(input string) (Contract, error) {
	var contract Contract
	for lineNumber, rawLine := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens, err := tokenize(line)
		if err != nil {
			return Contract{}, fmt.Errorf("line %d: %w", lineNumber+1, err)
		}
		if len(tokens) == 0 {
			continue
		}
		switch tokens[0] {
		case "gooo":
			if len(tokens) != 3 || tokens[1] != "error_directed_evolution_planner" || tokens[2] != "v1" {
				return Contract{}, fmt.Errorf("line %d: invalid header", lineNumber+1)
			}
			contract.Schema = ContractSchema
		case "authority":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: invalid authority", lineNumber+1)
			}
			contract.Authority = tokens[1]
		case "denominator":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.Denominator = DenominatorDecl{ID: values["id"], Scenarios: mustInt(values["scenarios"]), Unit: values["unit"]}
		case "decision":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.Statuses = splitCSV(values["statuses"])
		case "precedence":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: invalid precedence", lineNumber+1)
			}
			contract.Precedence = strings.Split(tokens[1], ">")
		case "unknown_fields":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: invalid unknown_fields", lineNumber+1)
			}
			contract.UnknownFields = strings.Split(tokens[1], ",")
		case "source_policy":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.SourcePolicy = values
		case "toolchain":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.Toolchain = ToolchainDecl{Go: values["go"], Digest: values["digest"]}
		case "operation":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.Operations = append(contract.Operations, OperationSpec{Ordinal: mustInt(values["ordinal"]), Kind: values["kind"], Target: values["target"], Cost: mustInt(values["cost"])})
		case "candidate_order":
			if len(tokens) != 2 {
				return Contract{}, fmt.Errorf("line %d: invalid candidate_order", lineNumber+1)
			}
			contract.CandidateOrder = splitCSV(tokens[1])
		case "generation_plan":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.GenerationPlan = GenerationPlan{Order: splitCSV(values["order"]), Outputs: splitCSV(values["outputs"])}
		case "scenario":
			values, err := keyValues(tokens[1:])
			if err != nil {
				return Contract{}, lineError(lineNumber, err)
			}
			contract.Scenarios = append(contract.Scenarios, ScenarioSpec{
				Ordinal: mustInt(values["ordinal"]), ID: values["id"], Terminal: values["terminal"], Counterexample: values["counterexample"], Guardrails: values["guardrails"], Expected: values["expected"],
				TerminalDigest: values["terminal_digest"], CounterexampleDigest: values["counterexample_digest"],
			})
		default:
			return Contract{}, fmt.Errorf("line %d: unknown declaration %q", lineNumber+1, tokens[0])
		}
	}
	return contract, nil
}

func (c Contract) Validate() error {
	if c.Schema != ContractSchema || c.Authority != "metacode" {
		return fmt.Errorf(".gooo must declare %s and metacode authority", ContractSchema)
	}
	if c.Denominator.ID == "" || c.Denominator.Scenarios != len(c.Scenarios) || c.Denominator.Unit != "scenario" {
		return fmt.Errorf("denominator does not match declared scenarios")
	}
	if !equalStrings(c.Statuses, []string{DecisionClosed, DecisionUnknown, DecisionRefuted}) || !equalStrings(c.Precedence, []string{DecisionRefuted, DecisionUnknown, DecisionClosed}) {
		return fmt.Errorf("statuses or precedence are not declared exactly")
	}
	if !equalStrings(c.UnknownFields, requiredUnknownFields) {
		return fmt.Errorf("UNKNOWN must declare the required six fields in order")
	}
	if c.SourcePolicy["terminal"] != "immutable_release" || c.SourcePolicy["counterexample"] != "immutable_release" || c.SourcePolicy["candidate"] != "caller_owned_temp" {
		return fmt.Errorf("source policy must bind immutable inputs and caller-owned candidate output")
	}
	if c.Toolchain.Go != "1.27" || !validDigest(c.Toolchain.Digest) {
		return fmt.Errorf("toolchain declaration is incomplete")
	}
	if len(c.Operations) != len(allowedOperations) {
		return fmt.Errorf("exactly four delta operations are required")
	}
	seenOps := map[string]bool{}
	for _, operation := range c.Operations {
		if !contains(allowedOperations, operation.Kind) || seenOps[operation.Kind] || operation.Ordinal < 1 || operation.Cost < 1 || operation.Target == "" {
			return fmt.Errorf("invalid or duplicate delta operation %q", operation.Kind)
		}
		seenOps[operation.Kind] = true
	}
	if !equalStrings(c.CandidateOrder, []string{"SPLIT", "ADD", "RETIRE", "REWIRE"}) {
		return fmt.Errorf("candidate order is not deterministic")
	}
	if len(c.GenerationPlan.Order) != len(c.GenerationPlan.Outputs) || len(c.GenerationPlan.Outputs) != 5 || c.GenerationPlan.Order[0] != "candidate-bundle.json" {
		return fmt.Errorf("generation plan is incomplete")
	}
	seenScenarios := map[string]bool{}
	for _, scenario := range c.Scenarios {
		if scenario.ID == "" || seenScenarios[scenario.ID] || scenario.Terminal == "" || scenario.Counterexample == "" || scenario.Guardrails == "" || !contains([]string{DecisionClosed, DecisionUnknown, DecisionRefuted}, scenario.Expected) || !validDigest(scenario.TerminalDigest) || !validDigest(scenario.CounterexampleDigest) {
			return fmt.Errorf("scenario %q is incomplete or duplicated", scenario.ID)
		}
		seenScenarios[scenario.ID] = true
	}
	return nil
}

func (c Contract) Scenario(id string) (ScenarioSpec, error) {
	for _, scenario := range c.Scenarios {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return ScenarioSpec{}, fmt.Errorf("scenario %q is not declared in .gooo", id)
}

func LoadTerminal(path string) (TerminalRecord, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return TerminalRecord{}, nil, err
	}
	var record TerminalRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return TerminalRecord{}, nil, fmt.Errorf("parse terminal record: %w", err)
	}
	if record.Schema != TerminalSchema || !contains([]string{DecisionClosed, DecisionUnknown, DecisionRefuted}, record.Decision) {
		return TerminalRecord{}, nil, fmt.Errorf("terminal record schema or decision is invalid")
	}
	if record.Decision == DecisionUnknown && !record.unknown().valid() {
		return TerminalRecord{}, nil, fmt.Errorf("terminal UNKNOWN does not carry six fields")
	}
	if record.Decision == DecisionRefuted && (!validDigest(record.CounterexampleDigest) || record.Reason == "") {
		return TerminalRecord{}, nil, fmt.Errorf("terminal REFUTED record lacks counterexample and reason")
	}
	return record, raw, nil
}

func LoadCounterexample(path string) (Counterexample, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Counterexample{}, nil, err
	}
	var counterexample Counterexample
	if err := json.Unmarshal(raw, &counterexample); err == nil && counterexample.Schema == CounterexampleSchema {
		if err := counterexample.Validate(); err != nil {
			return Counterexample{}, nil, err
		}
		return counterexample, raw, nil
	}
	var reducerReport struct {
		Schema          string `json:"schema"`
		Scenario        string `json:"scenario"`
		SourceDigest    string `json:"source_digest"`
		ToolchainDigest string `json:"toolchain_digest"`
		Baseline        struct {
			Decision     string `json:"decision"`
			Reason       string `json:"reason"`
			ReasonDigest string `json:"reason_digest"`
		} `json:"baseline"`
		ReducedGraph struct {
			Schema string      `json:"schema"`
			Nodes  []GraphNode `json:"nodes"`
			Edges  []GraphEdge `json:"edges"`
		} `json:"reduced_graph"`
	}
	if err := json.Unmarshal(raw, &reducerReport); err != nil {
		return Counterexample{}, nil, fmt.Errorf("parse counterexample: %w", err)
	}
	if reducerReport.Schema != "gooo.semantic_counterexample_reduction_report/v1" || reducerReport.Scenario == "" || reducerReport.Baseline.Decision != DecisionRefuted {
		return Counterexample{}, nil, fmt.Errorf("unsupported counterexample asset schema")
	}
	if !validDigest(reducerReport.Baseline.ReasonDigest) {
		reducerReport.Baseline.ReasonDigest = DigestString(reducerReport.Baseline.Reason)
	}
	counterexample = Counterexample{
		Schema: CounterexampleSchema, ID: reducerReport.Scenario, Scenario: reducerReport.Scenario, SourceDigest: reducerReport.SourceDigest, ToolchainDigest: reducerReport.ToolchainDigest,
		Decision: reducerReport.Baseline.Decision, Reason: reducerReport.Baseline.Reason, ReasonDigest: reducerReport.Baseline.ReasonDigest,
		MinimizedGraph: MinimizedGraph{Schema: reducerReport.ReducedGraph.Schema, Nodes: reducerReport.ReducedGraph.Nodes, Edges: reducerReport.ReducedGraph.Edges},
	}
	if err := counterexample.Validate(); err != nil {
		return Counterexample{}, nil, fmt.Errorf("reducer counterexample: %w", err)
	}
	return counterexample, raw, nil
}

func LoadGuardrails(path string) (GuardrailCorpus, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GuardrailCorpus{}, nil, err
	}
	var corpus GuardrailCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		return GuardrailCorpus{}, nil, err
	}
	if corpus.Schema != "gooo/error-directed-evolution-planner/guardrail-corpus/v1" || corpus.ID == "" || len(corpus.Cases) == 0 {
		return GuardrailCorpus{}, nil, fmt.Errorf("guardrail corpus is incomplete")
	}
	for _, testCase := range corpus.Cases {
		if testCase.ID == "" || !contains([]string{DecisionClosed, DecisionUnknown, DecisionRefuted}, testCase.BaselineDecision) || testCase.BaselineDecision != testCase.CandidateDecision {
			return GuardrailCorpus{}, nil, fmt.Errorf("guardrail %q is not a non-regression pair", testCase.ID)
		}
	}
	return corpus, raw, nil
}

func ReadFileDigest(path string) ([]byte, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return raw, Digest(raw), nil
}

func tokenize(line string) ([]string, error) {
	var tokens []string
	for index := 0; index < len(line); {
		for index < len(line) && (line[index] == ' ' || line[index] == '\t') {
			index++
		}
		if index == len(line) {
			break
		}
		var token strings.Builder
		if line[index] == '"' {
			start := index
			index++
			for index < len(line) {
				if line[index] == '\\' && index+1 < len(line) {
					index += 2
					continue
				}
				if line[index] == '"' {
					index++
					value, err := strconv.Unquote(line[start:index])
					if err != nil {
						return nil, err
					}
					tokens = append(tokens, value)
					break
				}
				index++
			}
			if index > len(line) || line[index-1] != '"' {
				return nil, fmt.Errorf("unterminated quoted token")
			}
			continue
		}
		for index < len(line) && line[index] != ' ' && line[index] != '\t' {
			token.WriteByte(line[index])
			index++
		}
		tokens = append(tokens, token.String())
	}
	return tokens, nil
}

func keyValues(tokens []string) (map[string]string, error) {
	values := map[string]string{}
	for _, token := range tokens {
		key, value, ok := strings.Cut(token, "=")
		if !ok || key == "" || values[key] != "" {
			return nil, fmt.Errorf("invalid key/value %q", token)
		}
		values[key] = value
	}
	return values, nil
}

func lineError(lineNumber int, err error) error { return fmt.Errorf("line %d: %w", lineNumber+1, err) }

func mustInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}
