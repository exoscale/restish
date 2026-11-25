package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rest-sh/restish/v2/plugin"
	"github.com/spf13/cobra"
)

const responseMiddlewareFlagAnnotation = "restish.responseMiddleware"

func (c *CLI) addResponseMiddlewareFlags(root *cobra.Command) error {
	flags := root.PersistentFlags()
	for _, p := range c.pluginsByHook["response-middleware"] {
		decl := p.Manifest.ResponseMiddlewareFlag
		if decl == nil {
			continue
		}
		name := "rsh-" + decl.Name
		for _, command := range commandTree(root) {
			if command.Flags().Lookup(name) != nil || command.PersistentFlags().Lookup(name) != nil {
				return fmt.Errorf("plugin %s: response middleware flag --%s conflicts with another flag", p.Manifest.Name, name)
			}
		}
		flags.Bool(name, false, decl.Usage)
		flags.Lookup(name).Annotations = map[string][]string{
			responseMiddlewareFlagAnnotation: {"true"},
			rshFlagGroupAnnotation:           {flagGroupOutput},
		}
	}
	return nil
}

func responseMiddlewareEnabled(gf GlobalFlags, manifest plugin.Manifest) bool {
	return manifest.ResponseMiddlewareFlag == nil ||
		slices.Contains(gf.ResponseMiddlewareFlags, "rsh-"+manifest.ResponseMiddlewareFlag.Name)
}

func (c *CLI) validateResponseMiddlewareCommand(cmd *cobra.Command, gf GlobalFlags) error {
	if len(gf.ResponseMiddlewareFlags) == 0 {
		return nil
	}
	names := "--" + strings.Join(gf.ResponseMiddlewareFlags, ", --")
	if gf.Verbose != 0 {
		return fmt.Errorf("%s cannot be combined with --rsh-verbose, which includes unprocessed request and response metadata", names)
	}
	if cmd == cmd.Root() || cmd.Annotations[requestHelpAnnotation] == "true" ||
		cmd.Annotations[generatedOperationAnnotation] == "true" || c.isAPIShortName(cmd.Name()) ||
		cmd.Name() == "__complete" || cmd.Name() == "__completeNoDesc" {
		return nil
	}
	return fmt.Errorf("%s does not support %s", cmd.CommandPath(), names)
}

func responseMiddlewareStreamError(gf GlobalFlags) error {
	if len(gf.ResponseMiddlewareFlags) == 0 {
		return nil
	}
	// shortcut: opt-in response hooks reject streams until per-record semantics exist.
	return fmt.Errorf("--%s does not support streaming responses", strings.Join(gf.ResponseMiddlewareFlags, ", --"))
}
