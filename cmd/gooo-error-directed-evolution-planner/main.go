package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-error-directed-evolution-planner/internal/planner"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("command is required: manifest, plan, or conformance"))
	}
	switch os.Args[1] {
	case "manifest":
		runManifest(os.Args[2:])
	case "plan":
		runPlan(os.Args[2:])
	case "conformance":
		runConformance(os.Args[2:])
	default:
		fail(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func commonFlags(name string, args []string) (*flag.FlagSet, *string, *string, *string, *string) {
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	contractPath := flags.String("contract", ".gooo/evolution-planner.gooo", "authoritative .gooo planner contract")
	repoRoot := flags.String("repo-root", ".", "read-only input repository root")
	outPath := flags.String("out", "", "caller-owned output directory")
	compiler := flags.String("compiler", "", "released compiler oracle executable")
	return flags, contractPath, repoRoot, outPath, compiler
}

func runManifest(args []string) {
	flags, contractPath, repoRoot, outPath, _ := commonFlags("manifest", args)
	flags.Parse(args)
	contract, raw, err := planner.LoadContract(*contractPath)
	if err != nil {
		fail(err)
	}
	if err := planner.EnsureCallerOutput(*outPath, *repoRoot); err != nil {
		fail(err)
	}
	if err := planner.WriteJSON(*outPath, planner.Manifest(contract, raw)); err != nil {
		fail(err)
	}
}

func runPlan(args []string) {
	flags, contractPath, repoRoot, outPath, compiler := commonFlags("plan", args)
	scenarioID := flags.String("scenario", "known-4-activity", "declared scenario id")
	terminalPath := flags.String("terminal", "", "optional immutable terminal record override")
	counterexamplePath := flags.String("counterexample", "", "optional immutable minimized counterexample override")
	guardrailsPath := flags.String("guardrails", "", "optional guardrail corpus override")
	baselinePath := flags.String("baseline-phase", "", "optional baseline .gooo phase override")
	flags.Parse(args)
	contract, raw, err := planner.LoadContract(*contractPath)
	if err != nil {
		fail(err)
	}
	scenario, err := contract.Scenario(*scenarioID)
	if err != nil {
		fail(err)
	}
	if err := planner.PrepareOutput(*outPath, *repoRoot); err != nil {
		fail(err)
	}
	_, err = planner.PlanScenario(planner.PlanOptions{ContractPath: *contractPath, Scenario: scenario, RepoRoot: *repoRoot, OutputDir: *outPath, Compiler: *compiler, TerminalPath: *terminalPath, CounterexamplePath: *counterexamplePath, GuardrailsPath: *guardrailsPath, BaselinePhasePath: *baselinePath}, contract, raw)
	if err != nil {
		fail(err)
	}
}

func runConformance(args []string) {
	flags, contractPath, repoRoot, outPath, compiler := commonFlags("conformance", args)
	flags.Parse(args)
	contract, raw, err := planner.LoadContract(*contractPath)
	if err != nil {
		fail(err)
	}
	report, err := planner.RunConformance(contract, raw, *repoRoot, *outPath, *compiler)
	if err != nil {
		fail(err)
	}
	if report.Failed != 0 {
		fail(fmt.Errorf("conformance failed: %d scenario(s)", report.Failed))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
