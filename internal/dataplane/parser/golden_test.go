package parser_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"testing"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/deckgen"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/parser"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/store"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"
)

var (
	updateGolden = flag.Bool("update-golden", false,
		"Updates golden files using the current output of the parser")
)

// TestParser_GoldenTests runs the golden tests for the parser.
// Every test case:
//   - is a directory in the ./testdata directory,
//   - has in.yaml file that represents the input with Kubernetes objects to be loaded into the store,
//   - has the out.yaml file that represents the expected output of the parser in the form of a Deck file.
//
// The test case is executed by loading the in.yaml file into the store, then running the parser on the store,
// and finally comparing the output of the parser with the out.yaml file.
//
// When adding a new test case, you can simply add a new directory with the in.yaml that you want to test against
// and run the test with the -update-golden flag to generate the out.yaml file that you can then verify and commit.
// You can use featureFlagsModifier to modify the feature flags used by the parser for the test case.
//
// If you introduce a change that may affect many test cases, and you're sure about it correctness, you can run the
// whole suite with the -update-golden flag as well to regenerate all golden files.
func TestParser_GoldenTests(t *testing.T) {
	flag.Parse()

	testCases := []struct {
		name                 string
		featureFlagsModifier func(flags *parser.FeatureFlags)
	}{
		{
			name: "global-plugin",
		},
		{
			name: "plugins-with-secret-configuration",
		},
		{
			name: "plugins-with-missing-secrets-or-keys",
		},
		{
			name: "plugins-with-both-config-and-configfrom",
		},
		{
			name: "pick-port-implicit",
		},
		{
			name: "pick-port-by-name",
		},
		{
			name: "ingress-v1-combined-routes-off",
			featureFlagsModifier: func(flags *parser.FeatureFlags) {
				flags.CombinedServiceRoutes = false
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k8sConfigFile := "testdata/" + tc.name + "/in.yaml"
			goldenFile := "testdata/" + tc.name + "/out.yaml"

			// Default feature flags.
			featureFlags := parser.FeatureFlags{
				ReportConfiguredKubernetesObjects: true,
				CombinedServiceRoutes:             true,
				RegexPathPrefix:                   true,
			}

			// Apply test case's feature flags modifier if defined.
			if tc.featureFlagsModifier != nil {
				tc.featureFlagsModifier(&featureFlags)
			}

			runParserGoldenTest(t, featureFlags, k8sConfigFile, goldenFile)
		})
	}
}

func runParserGoldenTest(t *testing.T, flags parser.FeatureFlags, k8sConfigFile string, goldenFile string) {
	logger := logrus.New()

	// Load the K8s objects from the YAML file.
	objects := extractObjectsFromYAML(t, k8sConfigFile)
	t.Logf("Found %d K8s objects to be loaded into the store", len(objects))

	// Load the K8s objects into the store.
	cacheStores, err := store.NewCacheStoresFromObjYAMLIgnoreUnknown(objects...)
	require.NoError(t, err, "Failed creating cache stores")

	// Create the parser.
	s := store.New(cacheStores, "kong", logger)
	p, err := parser.NewParser(logger, s, flags)
	require.NoError(t, err, "Failed creating parser")

	// Build the Kong configuration.
	result := p.BuildKongConfig()
	targetConfig := deckgen.ToDeckContent(context.Background(),
		logger,
		result.KongState,
		deckgen.GenerateDeckContentParams{
			FormatVersion:    "3.0",
			ExpressionRoutes: flags.ExpressionRoutes,
			PluginSchemas:    pluginsSchemaStoreStub{},
		},
	)

	// Marshal the result into YAML bytes for comparison.
	resultB, err := yaml.Marshal(targetConfig)
	require.NoError(t, err, "Failed marshalling result")

	// If the update flag is set, update the golden file with the result...
	if updateGolden != nil && *updateGolden {
		err = os.WriteFile(goldenFile, resultB, 0644)
		require.NoError(t, err, "Failed writing to golden file")
		t.Logf("Updated golden file %s", goldenFile)
	} else {
		// ...otherwise, compare the result to the golden file.
		goldenB, err := os.ReadFile(goldenFile)
		require.NoError(t, err, "Failed reading golden file")

		require.Equalf(t, string(goldenB), string(resultB),
			"Golden file %s does not match the result. \n"+
				"If you are sure the result is correct, run the test "+
				"with the -update-golden flag to update the golden file.",
			goldenFile)
		t.Logf("Successfully compared result to golden file %s", goldenFile)
	}
}

func extractObjectsFromYAML(t *testing.T, filePath string) [][]byte {
	y, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed reading input file")

	// Strip out the YAML comments.
	f := util.ManualStrip(y)

	// Split the YAML by the document separator.
	objects := bytes.Split(f, []byte("---"))

	// Filter out empty YAML documents.
	return lo.Filter(objects, func(o []byte, _ int) bool {
		return len(bytes.TrimSpace(o)) > 0
	})
}

// pluginsSchemaStoreStub is a stub implementation of the plugins.SchemaStore interface that returns an empty schema
// for all plugins. It's used to avoid hitting the Kong Admin API during tests.
type pluginsSchemaStoreStub struct{}

func (p pluginsSchemaStoreStub) Schema(context.Context, string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
