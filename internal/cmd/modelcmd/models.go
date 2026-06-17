package modelcmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// Models runs `modelgo models`: GET /v1/models, listing the model ids the API
// key can reach.
func Models(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printModelsUsage(stdout)
			return 0
		}
	}

	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := registerCommon(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client, err := cf.client()
	if err != nil {
		return fail(stderr, "models", err)
	}

	resp, err := client.Do(context.Background(), http.MethodGet, "/v1/models", nil, nil)
	if err != nil {
		return fail(stderr, "models", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fail(stderr, "models", err)
	}
	if *cf.jsonOut {
		stdout.Write(ensureTrailingNewline(data))
		return 0
	}

	var parsed struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fail(stderr, "models", fmt.Errorf("decode response: %w", err))
	}
	if len(parsed.Data) == 0 {
		fmt.Fprintln(stdout, "No models available.")
		return 0
	}
	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintln(stdout, id)
	}
	return 0
}

func printModelsUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo models — list available models (OpenAI-compatible /v1/models)

USAGE:
    modelgo models [flags]

FLAGS:
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path
    --json              Print the raw JSON response`)
}
