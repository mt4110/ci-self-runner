package main

import (
	"errors"
	"fmt"
)

func parseCLI(args []string) (cliCommand, error) {
	if len(args) == 0 {
		return cliCommand{kind: "run-plan", timeboxMin: 20}, nil
	}
	if args[0] == "-h" || args[0] == "--help" {
		return cliCommand{kind: "help"}, nil
	}

	cmd := cliCommand{timeboxMin: 20}
	switch args[0] {
	case string(stepPreflight):
		cmd.kind = "one-step"
		cmd.targetStep = stepPreflight
	case string(stepVerifyLite):
		cmd.kind = "one-step"
		cmd.targetStep = stepVerifyLite
	case string(stepFullBuild):
		cmd.kind = "one-step"
		cmd.targetStep = stepFullBuild
	case string(stepFullTest):
		cmd.kind = "one-step"
		cmd.targetStep = stepFullTest
	case string(stepBundleMake):
		cmd.kind = "one-step"
		cmd.targetStep = stepBundleMake
	case string(stepPrCreate):
		cmd.kind = "one-step"
		cmd.targetStep = stepPrCreate
	case "run-plan":
		cmd.kind = "run-plan"
	default:
		return cliCommand{}, fmt.Errorf("unknown command: %s", args[0])
	}

	i := 1
	for i < len(args) {
		switch args[i] {
		case "-h", "--help":
			return cliCommand{kind: "help"}, nil
		case "--timebox-min":
			if i+1 >= len(args) {
				return cliCommand{}, errors.New("missing value for --timebox-min")
			}
			v, convErr := parsePositiveUint(args[i+1])
			if convErr != nil {
				return cliCommand{}, errors.New("invalid --timebox-min value")
			}
			cmd.timeboxMin = v
			i += 2
		default:
			return cliCommand{}, fmt.Errorf("unexpected argument: %s", args[i])
		}
	}
	return cmd, nil
}

func parsePositiveUint(raw string) (uint64, error) {
	var v uint64
	_, err := fmt.Sscanf(raw, "%d", &v)
	if err != nil || v == 0 {
		return 0, errors.New("invalid value")
	}
	return v, nil
}

func printHelp() {
	fmt.Println("ci_orch commands:")
	fmt.Println("  preflight")
	fmt.Println("  verify-lite")
	fmt.Println("  full-build")
	fmt.Println("  full-test")
	fmt.Println("  bundle-make")
	fmt.Println("  pr-create")
	fmt.Println("  run-plan")
	fmt.Println("options:")
	fmt.Println("  --timebox-min <N>")
}
