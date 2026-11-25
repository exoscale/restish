package plugin

import (
	"strings"
	"testing"

	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func TestValidateResponseMiddlewareFlag(t *testing.T) {
	type testCase struct {
		name   string
		mutate func(*Manifest)
		want   string
	}
	tests := []testCase{
		{"valid", func(*Manifest) {}, ""},
		{"missing declaration", func(m *Manifest) { m.ResponseMiddlewareFlag = nil }, "omits response_middleware_flag"},
		{"missing feature", func(m *Manifest) { m.RequiredFeatures = nil }, "in required_features"},
		{"missing hook", func(m *Manifest) { m.Hooks = []string{"auth"} }, "without declaring response-middleware hook"},
		{"missing usage", func(m *Manifest) { m.ResponseMiddlewareFlag.Usage = " " }, "omits usage"},
	}
	for _, name := range []string{"", "-bad", "bad-", "bad--name", "Bad", "bad_name", "bad name", "bad/name", "bad=true", "bäd"} {
		tests = append(tests, testCase{"invalid name " + name, func(m *Manifest) { m.ResponseMiddlewareFlag.Name = name }, "must use lowercase"})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := Manifest{
				Name: "test", RestishAPIVersion: 2,
				Hooks: []string{"response-middleware"}, RequiredFeatures: []string{pluginwire.FeatureResponseMiddlewareFlag},
				ResponseMiddlewareFlag: &pluginwire.ResponseMiddlewareFlag{Name: "test-2", Usage: "Transform responses"},
			}
			test.mutate(&manifest)
			err := validateManifest(manifest)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
