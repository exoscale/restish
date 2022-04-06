package cli

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func reset(color bool) {
	viper.Reset()

	if color {
		viper.Set("color", true)
	} else {
		viper.Set("nocolor", true)
	}

	Init("test", "1.0.0'")
	Defaults()
}

func run(cmd string, color ...bool) string {
	if len(color) == 0 || !color[0] {
		reset(false)
	} else {
		reset(true)
	}

	return runNoReset(cmd)
}

func runNoReset(cmd string) string {
	capture := &strings.Builder{}
	Stdout = capture
	Stderr = capture
	Root.SetOut(capture)
	os.Args = strings.Split("restish "+cmd, " ")
	Run()

	return capture.String()
}

func expectJSON(t *testing.T, cmd string, expected string) {
	captured := run("-o json -f body " + cmd)
	assert.JSONEq(t, expected, captured)
}

func TestGetURI(t *testing.T) {
	defer gock.Off()

	gock.New("http://example.com").Get("/foo").Reply(200).JSON(map[string]interface{}{
		"Hello": "World",
	})

	expectJSON(t, "http://example.com/foo", `{
		"Hello": "World"
	}`)
}

func TestPostURI(t *testing.T) {
	defer gock.Off()

	gock.New("http://example.com").Post("/foo").Reply(200).JSON(map[string]interface{}{
		"id":    1,
		"value": 123,
	})

	expectJSON(t, "post http://example.com/foo value: 123", `{
		"id": 1,
		"value": 123
	}`)
}

func TestPutURI400(t *testing.T) {
	defer gock.Off()

	gock.New("http://example.com").Put("/foo/1").Reply(400).JSON(map[string]interface{}{
		"detail": "Invalid input",
	})

	expectJSON(t, "put http://example.com/foo/1 value: 123", `{
		"detail": "Invalid input"
	}`)
}

type TestAuth struct{}

// Parameters returns a list of OAuth2 Authorization Code inputs.
func (h *TestAuth) Parameters() []AuthParam {
	return []AuthParam{}
}

// OnRequest gets run before the request goes out on the wire.
func (h *TestAuth) OnRequest(request *http.Request, key string, params map[string]string) error {
	request.Header.Set("Authorization", "abc123")
	return nil
}

func TestAuthHeader(t *testing.T) {
	reset(false)

	AddAuth("test-auth", &TestAuth{})

	configs["test-auth-header"] = &APIConfig{
		name: "test-auth-header",
		Base: "https://auth-header-test.example.com",
		Profiles: map[string]*APIProfile{
			"default": {
				Auth: &APIAuth{
					Name: "test-auth",
				},
			},
			"no-auth": {},
		},
	}

	captured := runNoReset("auth-header bad-api")
	assert.Contains(t, captured, "No matched API")

	captured = runNoReset("auth-header test-auth-header")
	assert.Equal(t, "abc123\n", captured)

	captured = runNoReset("auth-header test-auth-header -p bad")
	assert.Contains(t, captured, "Invalid profile bad")

	captured = runNoReset("auth-header test-auth-header -p no-auth")
	assert.Contains(t, captured, "No auth set up")
}

func TestLinks(t *testing.T) {
	defer gock.Off()

	gock.New("http://example.com").Get("/foo").Reply(204).SetHeader("Link", "</bar>; rel=\"item\"")

	captured := run("links http://example.com/foo")
	assert.JSONEq(t, `{
		"item": [
			{
				"rel": "item",
				"uri": "http://example.com/bar"
			}
		]
	}`, captured)
}

func TestDefaultOutput(t *testing.T) {
	defer gock.Off()

	gock.New("http://example.com").Get("/foo").Reply(200).JSON(map[string]interface{}{
		"hello": "world",
	})

	captured := run("http://example.com/foo", true)
	assert.Equal(t, "\x1b[38;5;204mHTTP\x1b[0m/\x1b[38;5;172m1.1\x1b[0m \x1b[38;5;172m200\x1b[0m \x1b[38;5;74mOK\x1b[0m\n\x1b[38;5;74mContent-Type\x1b[0m: application/json\n\n\x1b[38;5;247m{\x1b[0m\n  \x1b[38;5;74mhello\x1b[0m\x1b[38;5;247m:\x1b[0m \x1b[38;5;150m\"world\"\x1b[0m\x1b[38;5;247m\n}\x1b[0m\n", captured)
}

func TestHelp(t *testing.T) {
	captured := run("--help", false)
	assert.Contains(t, captured, "api")
	assert.Contains(t, captured, "get")
	assert.Contains(t, captured, "put")
	assert.Contains(t, captured, "delete")
	assert.Contains(t, captured, "edit")
}

func TestHelpHighlight(t *testing.T) {
	captured := run("--help", true)
	assert.Contains(t, captured, "api")
	assert.Contains(t, captured, "get")
	assert.Contains(t, captured, "put")
	assert.Contains(t, captured, "delete")
	assert.Contains(t, captured, "edit")
}

func TestLoadCache(t *testing.T) {
	// Invalidate any existin cache.
	Cache.Set("cache-test.expires", time.Now().Add(-24*time.Hour))
	Cache.WriteConfig()
	defer gock.Off()

	// Only *one* set of remote requests should be made. After that it should be
	// using the cache.
	gock.New("https://example.com/").Reply(404)
	gock.New("https://example.com/openapi.json").Reply(200).JSON(map[string]interface{}{
		"openapi": "3.0.0",
	})

	reset(false)
	configs["cache-test"] = &APIConfig{
		name: "cache-test",
		Base: "https://example.com",
		Profiles: map[string]*APIProfile{
			"default": {},
		},
	}
	cmd := &cobra.Command{
		Use: "cache-test",
	}
	Root.AddCommand(cmd)

	AddLoader(&testLoader{
		API: API{
			Short:      "Cache Test API",
			Operations: []Operation{},
		},
	})

	// First run will generate the cache.
	runNoReset("cache-test --help")

	// These runs should *not* make any remote requests. If they do, then
	// gock will panic as only one call is mocked above.
	runNoReset("cache-test --help")
	runNoReset("cache-test --help")
}

func TestAPISync(t *testing.T) {
	defer gock.Off()

	gock.New("https://sync-test.example.com/").Reply(404)
	gock.New("https://sync-test.example.com/openapi.json").Reply(404)

	reset(false)

	configs["sync-test"] = &APIConfig{
		name: "sync-test",
		Base: "https://sync-test.example.com",
		Profiles: map[string]*APIProfile{
			"default": {},
		},
	}

	runNoReset("api sync sync-test")
}
