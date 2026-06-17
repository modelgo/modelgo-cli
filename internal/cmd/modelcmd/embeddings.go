package modelcmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Embeddings runs `modelgo embeddings`: POST /v1/embeddings. Input text comes
// from positional args or stdin.
func Embeddings(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printEmbeddingsUsage(stdout)
			return 0
		}
	}

	fs := flag.NewFlagSet("embeddings", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := registerCommon(fs)
	model := fs.String("model", "", "embedding model id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *model == "" {
		fmt.Fprintln(stderr, "embeddings: --model is required")
		return 2
	}

	input, err := readPrompt(fs.Args(), stdin)
	if err != nil {
		return fail(stderr, "embeddings", err)
	}

	payload, _ := json.Marshal(map[string]any{"model": *model, "input": input})
	client, err := cf.client()
	if err != nil {
		return fail(stderr, "embeddings", err)
	}

	resp, err := client.Do(context.Background(), http.MethodPost, "/v1/embeddings", strings.NewReader(string(payload)), nil)
	if err != nil {
		return fail(stderr, "embeddings", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fail(stderr, "embeddings", err)
	}
	if *cf.jsonOut {
		stdout.Write(ensureTrailingNewline(data))
		return 0
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Model string `json:"model"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fail(stderr, "embeddings", fmt.Errorf("decode response: %w", err))
	}
	if len(parsed.Data) == 0 {
		fmt.Fprintln(stdout, "No embedding returned.")
		return 0
	}
	dims := len(parsed.Data[0].Embedding)
	fmt.Fprintf(stdout, "model:  %s\n", parsed.Model)
	fmt.Fprintf(stdout, "vectors: %d\n", len(parsed.Data))
	fmt.Fprintf(stdout, "dims:    %d\n", dims)
	fmt.Fprintf(stdout, "tokens:  %d\n", parsed.Usage.TotalTokens)
	fmt.Fprintln(stdout, "Pass --json for the full embedding vectors.")
	return 0
}

func printEmbeddingsUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo embeddings — create embeddings (OpenAI-compatible /v1/embeddings)

USAGE:
    modelgo embeddings --model MODEL [flags] [TEXT...]
    echo "TEXT" | modelgo embeddings --model MODEL

FLAGS:
    --model MODEL       Embedding model id (required)
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path
    --json              Print the raw JSON response (full vectors)`)
}
