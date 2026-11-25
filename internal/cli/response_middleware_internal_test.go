package cli

import (
	"context"
	"strings"
	"testing"

	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestResponseMiddlewareFlagCollisions(t *testing.T) {
	for _, name := range []string{"raw", "transform", "generate-body"} {
		t.Run(name, func(t *testing.T) {
			c := New()
			manifest := plugin.Manifest{
				Name: "first", Hooks: []string{"response-middleware"},
				ResponseMiddlewareFlag: &plugin.ResponseMiddlewareFlag{Name: name, Usage: "Transform"},
			}
			plugins := []internalplugin.Plugin{{Manifest: manifest}}
			if name == "transform" {
				manifest.Name = "second"
				plugins = append(plugins, internalplugin.Plugin{Manifest: manifest})
			}
			c.pluginsByHook = map[string][]internalplugin.Plugin{"response-middleware": plugins}
			root := c.newRootCmd()
			if name == "generate-body" {
				child := &cobra.Command{Use: "operation"}
				child.Flags().Bool("rsh-generate-body", false, "Print an example request body")
				root.AddCommand(child)
			}
			if err := c.addResponseMiddlewareFlags(root); err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("error = %v, want flag conflict", err)
			}
		})
	}
}

func TestResponseMiddlewareFlagsDoNotConsumeControlArguments(t *testing.T) {
	scan := scanCLIArgs([]string{
		"restish", "--rsh-anonymize", "--rsh-config", "test.json", "--rsh-profile", "test",
		"api", "sync", "example",
	})
	if scan.ConfigPath != "test.json" || scan.ProfileName != "test" || scan.FirstCommand != "api" || scan.SecondCommand != "sync" {
		t.Fatalf("scan = %#v", scan)
	}
	root := New().newRootCmd()
	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if strings.HasPrefix(flag.Name, "rsh-") && flagConsumesNextArg("--"+flag.Name) != (flag.NoOptDefVal == "") {
			t.Errorf("--%s scan arity differs from its registered flag", flag.Name)
		}
	})
}

func TestCommandPluginRejectsResponseMiddlewareFlags(t *testing.T) {
	c := New()
	c.pluginsByHook = map[string][]internalplugin.Plugin{"response-middleware": {{
		Manifest: plugin.Manifest{
			Name:                   "test",
			ResponseMiddlewareFlag: &plugin.ResponseMiddlewareFlag{Name: "anonymize", Usage: "Transform"},
		},
	}}}
	root := c.newRootCmd()
	if err := c.addResponseMiddlewareFlags(root); err != nil {
		t.Fatal(err)
	}
	root.SetContext(context.Background())
	for _, arg := range []string{"--rsh-anonymize", "--rsh-anonymize=true", "--rsh-anonymize=invalid"} {
		err := c.runCommandPlugin(root, "must-not-start", plugin.CommandDecl{}, []string{arg})
		if err == nil || !strings.Contains(err.Error(), "rsh-anonymize") {
			t.Fatalf("%s: error = %v", arg, err)
		}
	}
}
