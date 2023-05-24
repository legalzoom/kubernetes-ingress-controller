package golden_test

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
	updateGolden = flag.Bool("update-golden", false, "Update golden files")
)

func TestParser_GoldenTests(t *testing.T) {
	flag.Parse()

	testCases := []struct {
		name string
	}{
		{
			name: "simple",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k8sConfigFile := "testdata/" + tc.name + "/in.yaml"
			goldenFile := "testdata/" + tc.name + "/out.yaml"

			runParserGoldenTest(t, k8sConfigFile, goldenFile)
		})
	}
}

func runParserGoldenTest(t *testing.T, k8sConfigFile string, goldenFile string) {
	var files [][]byte
	y, err := os.ReadFile(k8sConfigFile)
	require.NoError(t, err, "Failed reading input file")

	files = append(files, y)
	var objects [][]byte
	for _, f := range files {
		f := util.ManualStrip(f)
		objects = append(objects, splitYAMLs(f)...)
		objects = lo.Filter(objects, func(yaml []byte, _ int) bool {
			return len(yaml) > 0 && len(bytes.TrimSpace(yaml)) > 0
		})
	}

	t.Logf("Found %d K8s objects to be parsed", len(objects))

	logger := logrus.New()
	cacheStores, err := store.NewCacheStoresFromObjYAMLIgnoreUnknown(objects...)
	require.NoError(t, err, "Failed creating cache stores")

	s := store.New(cacheStores, "kong", logger)
	p, err := parser.NewParser(logger, s, parser.FeatureFlags{
		ReportConfiguredKubernetesObjects: true,
		CombinedServiceRoutes:             true,
		RegexPathPrefix:                   true,
	})
	require.NoError(t, err, "Failed creating parser")

	result := p.BuildKongConfig()
	if len(result.TranslationFailures) > 0 {
		t.Logf("%d failures occurred while building KongState", len(result.TranslationFailures))
	}

	targetConfig := deckgen.ToDeckContent(context.Background(),
		logger,
		result.KongState,
		deckgen.GenerateDeckContentParams{
			FormatVersion:    "3.0",
			SelectorTags:     nil,
			ExpressionRoutes: false,
			PluginSchemas:    pluginsSchemaStoreStub{},
		},
	)

	resultB, err := yaml.Marshal(targetConfig)
	require.NoError(t, err, "Failed marshalling result")

	if updateGolden != nil && *updateGolden {
		err = os.WriteFile(goldenFile, resultB, 0644)
		require.NoError(t, err, "Failed writing to golden file")
	} else {
		goldenB, err := os.ReadFile(goldenFile)
		require.NoError(t, err, "Failed reading golden file")
		require.Equal(t, string(goldenB), string(resultB), "Golden file does not match result")
	}
}

type pluginsSchemaStoreStub struct{}

func (p pluginsSchemaStoreStub) Schema(context.Context, string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func splitYAMLs(yamls []byte) [][]byte {
	return bytes.Split(yamls, []byte("---"))
}
