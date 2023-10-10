package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// DependenciesFile is a map of test levels and their dependencies.
type DependenciesFile map[string]Dependencies

// Dependencies is a map of dependencies and their versions (either a single value or a slice of them).
// Instead of versions, map values can also be a slice of matrix entries that can be used in GitHub Actions matrix
// directly.
type Dependencies map[string]interface{}

// This program reads `.github/test_dependencies.yaml` file, extracts a requested dependency's versions
// from it and prints it to stdout as a JSON value (as an array or a single value, depending on YAML definition).
//
// Usage:
// go run scripts/test-deps/main.go <test-level> <dependency> [--latest]
func main() {
	b, err := os.ReadFile(".github/test_dependencies.yaml")
	exitIfErr(err)

	deps := DependenciesFile{}
	err = yaml.Unmarshal(b, &deps)
	exitIfErr(err)

	testLevel := ""
	flag.StringVar(&testLevel, "test-level", "", "Test level to extract dependencies for.")

	dependency := ""
	flag.StringVar(&dependency, "dependency", "", "Dependency to extract versions for.")

	latest := false
	flag.BoolVar(&latest, "latest", false, "If set, only the latest version will be used.")

	flag.Parse()

	// Extract the requested dependency.
	testLevelDeps, ok := deps[testLevel]
	if !ok {
		exitWithErr(fmt.Errorf("test level %s not found", testLevel))
	}

	dependencyEntry, ok := testLevelDeps[dependency]
	if !ok {
		exitWithErr(fmt.Errorf("dependency %s.%s not found", testLevel, dependency))
	}

	jsonValue, err := getJSONValueForDependency(dependencyEntry, latest)
	if err != nil {
		exitWithErr(fmt.Errorf("failed to get JSON value for dependency %s.%s: %w", testLevel, dependency, err))
	}

	if err := json.NewEncoder(os.Stdout).Encode(jsonValue); err != nil {
		exitWithErr(fmt.Errorf("failed to encode JSON value for dependency %s.%s: %w", testLevel, dependency, err))
	}
}

func getJSONValueForDependency(dependencyEntry any, latest bool) (any, error) {
	switch v := dependencyEntry.(type) {
	case string:
		return v, nil
	case []any:
		// If --latest, use only the latest version (we assume latest is the first entry).
		if latest {
			v = []any{v[0]}
		}
		return v, nil
	default:
		return nil, fmt.Errorf("dependency entry of unsupported type: %T", dependencyEntry)
	}
}

func exitIfErr(err error) {
	if err != nil {
		exitWithErr(err)
	}
}

func exitWithErr(err error) {
	fmt.Printf("ERROR: %s\n", err)
	os.Exit(1)
}
