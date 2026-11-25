//go:build integration

package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func newFlaggedHookCLI(t *testing.T) (*cli.CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_RESPONSE_FLAG", "test-response")
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "flagged")
	c, out, diagnostic := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	c.Hooks().PluginManifestCachePath = filepath.Join(t.TempDir(), "manifest.cbor")
	return c, out, diagnostic
}

func TestResponseMiddlewareFlagActivation(t *testing.T) {
	c, out, diagnostic := newFlaggedHookCLI(t)
	srv := hookJSONServer(t, 200, `{"id":"private-value","count":7}`)
	for _, args := range [][]string{
		{"restish", srv.URL},
		{"restish", srv.URL, "--rsh-test-response=false"},
		{"restish", srv.URL, "--rsh-test-response"},
		{"restish", "--rsh-test-response", srv.URL},
		{"restish", srv.URL},
		{"restish", "get", "--help-all"},
	} {
		out.Reset()
		diagnostic.Reset()
		if err := c.Run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		want := "private-value"
		if strings.Contains(strings.Join(args, " "), "--help-all") {
			want = "--rsh-test-response"
		} else if strings.Contains(" "+strings.Join(args, " ")+" ", " --rsh-test-response ") {
			want = "masked"
			if strings.Contains(out.String(), "private-value") {
				t.Fatalf("%v leaked the original response: %s", args, out)
			}
		}
		if !strings.Contains(out.String(), want) || strings.Contains(diagnostic.String(), "unsupported feature") {
			t.Fatalf("%v: stdout=%q stderr=%q, want %q", args, out, diagnostic, want)
		}
	}
}

func TestResponseMiddlewareFlagRejectsUnsupportedCommandsAndModes(t *testing.T) {
	c, out, _ := newFlaggedHookCLI(t)
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"id":"private-value"}`)
	}))
	t.Cleanup(srv.Close)
	for _, args := range [][]string{
		{"restish", srv.URL, "--rsh-test-response", "-r"},
		{"restish", srv.URL, "--rsh-test-response", "-v"},
		{"restish", "edit", srv.URL, "--rsh-test-response", "--no-editor"},
		{"restish", "doctor", "--rsh-test-response"},
		{"restish", "version", "--rsh-test-response"},
	} {
		out.Reset()
		err := c.Run(args)
		if err == nil || !strings.Contains(err.Error(), "rsh-test-response") {
			t.Fatalf("%v: error = %v", args, err)
		}
		if out.Len() != 0 || requests.Load() != 0 {
			t.Fatalf("%v: output=%q requests=%d", args, out, requests.Load())
		}
	}
	t.Setenv("RSH_HOOK_RESPONSE_FLAG", "")
	if err := c.Run([]string{"restish", srv.URL, "--rsh-test-response"}); err == nil {
		t.Fatal("missing plugin flag must not silently proceed")
	}
	if requests.Load() != 0 {
		t.Fatal("missing plugin flag issued a request")
	}
}

func TestResponseMiddlewareFlagRejectsStreams(t *testing.T) {
	for _, contentType := range []string{"application/x-ndjson", "text/event-stream", "application/json"} {
		t.Run(contentType, func(t *testing.T) {
			c, out, _ := newFlaggedHookCLI(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				fmt.Fprintln(w, `{"id":"private-value"}`)
			}))
			t.Cleanup(srv.Close)
			err := c.Run([]string{"restish", srv.URL, "--rsh-test-response", "--rsh-max-items", "1"})
			if err == nil || !strings.Contains(err.Error(), "does not support streaming") {
				t.Fatalf("error = %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("stream data escaped middleware: %q", out)
			}
		})
	}
}

func TestResponseMiddlewareFlagAppliesToEveryPage(t *testing.T) {
	for _, test := range []struct {
		behavior string
		format   string
		masked   int
		wantErr  string
	}{
		{"flagged", "", 2, ""},
		{"flagged", "lines", 2, ""},
		{"flagged-fail-page", "", 0, "test response middleware failure"},
		{"flagged-fail-page", "lines", 1, "test response middleware failure"},
		{"flagged-drop-page", "", 0, ""},
		{"flagged-drop-page", "lines", 1, ""},
		{"flagged-follow-page", "", 0, "cannot follow another request"},
		{"flagged-follow-page", "lines", 1, "cannot follow another request"},
	} {
		t.Run(test.behavior+"/"+test.format, func(t *testing.T) {
			c, out, _ := newFlaggedHookCLI(t)
			t.Setenv("RSH_HOOK_RM_BEHAVIOR", test.behavior)
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("page") != "2" {
					w.Header().Set("Link", `</items?page=2>; rel="next"`)
				}
				fmt.Fprint(w, `[{"id":"private-value"}]`)
			}))
			t.Cleanup(srv.Close)
			args := []string{"restish", srv.URL + "/items", "--rsh-test-response"}
			if test.format != "" {
				args = append(args, "-o", test.format, "-f", "body.id")
			}
			err := c.Run(args)
			if test.wantErr == "" && err != nil || test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if strings.Contains(out.String(), "private-value") || strings.Count(out.String(), "masked") != test.masked || requests.Load() != 2 {
				t.Fatalf("output=%q requests=%d, want %d masked records and 2 requests", out, requests.Load(), test.masked)
			}
		})
	}
}

func TestResponseMiddlewareFlagAppliesAfterFollow(t *testing.T) {
	for _, loop := range []bool{false, true} {
		t.Run(fmt.Sprintf("loop=%t", loop), func(t *testing.T) {
			c, out, _ := newFlaggedHookCLI(t)
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"private-value"}`)
			}))
			t.Cleanup(srv.Close)
			behavior := "flagged-follow"
			if loop {
				behavior = "follow:" + srv.URL + "/final"
			}
			t.Setenv("RSH_HOOK_RM_BEHAVIOR", behavior)
			err := c.Run([]string{"restish", srv.URL + "/start", "--rsh-test-response"})
			if loop {
				if err == nil || !strings.Contains(err.Error(), "cannot follow another request") {
					t.Fatalf("follow loop error = %v", err)
				}
			} else if err != nil || !strings.Contains(out.String(), "masked") {
				t.Fatalf("error=%v output=%q", err, out)
			}
			if strings.Contains(out.String(), "private-value") || requests.Load() != 2 {
				t.Fatalf("output=%q requests=%d", out, requests.Load())
			}
		})
	}
}

func TestResponseMiddlewareFlagWithGeneratedCommandAndExplicitConfig(t *testing.T) {
	c, out, _ := newFlaggedHookCLI(t)
	srv := hookJSONServer(t, 200, `{"id":"private-value"}`)
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	spec := `{"openapi":"3.1.0","info":{"title":"Test","version":"1"},"paths":{"/items":{"get":{"operationId":"listItems","responses":{"200":{"description":"OK"}}}}}}`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`{"apis":{"tapi":{"base_url":%q,"spec_files":[%q]}}}`, srv.URL, specPath)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Run([]string{"restish", "--rsh-test-response", "--rsh-config", c.Hooks().ConfigPath, "tapi", "list-items"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "private-value") || !strings.Contains(out.String(), "masked") {
		t.Fatalf("generated response = %q", out)
	}
}
