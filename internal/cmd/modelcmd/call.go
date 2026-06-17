package modelcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// headerList collects repeated --header K:V flags.
type headerList []string

func (l *headerList) String() string { return strings.Join(*l, ",") }
func (l *headerList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// Call runs `modelgo call <path>`: a raw passthrough to any /v1/* gateway
// endpoint. It covers everything the convenience commands don't — images,
// audio, anthropic /v1/messages, multimodal embeddings, rerank, etc. The
// request body is read from --data, --data-file, or stdin; the response body is
// written verbatim to stdout.
func Call(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printCallUsage(stdout)
			return 0
		}
	}

	// The /v1/* path is positional but commonly precedes flags
	// (`modelgo call /v1/x --data ...`). Go's flag package stops at the first
	// non-flag token, so move flags ahead of positionals first, then parse.
	args = hoistFlags(args)

	fs := flag.NewFlagSet("call", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := registerCommon(fs) // --json is accepted but ignored: call is verbatim passthrough
	method := fs.String("method", "", "HTTP method (default POST with a body, GET without)")
	data := fs.String("data", "", "request body as a literal JSON string")
	dataFile := fs.String("data-file", "", "request body from a file (\"-\" = stdin)")
	stream := fs.Bool("stream", false, "stream the response body as it arrives")
	var headers headerList
	fs.Var(&headers, "header", "extra request header \"Key: Value\" (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(stderr, "call: a /v1/* path is required (e.g. modelgo call /v1/images/generations ...)")
		return 2
	}
	path := rest[0]

	hasBody := *data != "" || *dataFile != ""
	m := strings.ToUpper(*method)
	if m == "" {
		if hasBody {
			m = http.MethodPost
		} else {
			m = http.MethodGet
		}
	}

	var bodyReader io.Reader
	if hasBody || (m != http.MethodGet && m != http.MethodHead) {
		body, err := readData(*data, *dataFile, stdin)
		if err != nil {
			return fail(stderr, "call", err)
		}
		if len(body) > 0 {
			bodyReader = strings.NewReader(string(body))
		}
	}

	hdrs := map[string]string{}
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			fmt.Fprintf(stderr, "call: invalid --header %q (want \"Key: Value\")\n", h)
			return 2
		}
		hdrs[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	client, err := cf.client()
	if err != nil {
		return fail(stderr, "call", err)
	}

	resp, err := client.Do(context.Background(), m, path, bodyReader, hdrs)
	if err != nil {
		return fail(stderr, "call", err)
	}
	defer resp.Body.Close()

	if *stream {
		if _, err := io.Copy(stdout, resp.Body); err != nil {
			return fail(stderr, "call", err)
		}
		return 0
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return fail(stderr, "call", err)
	}
	stdout.Write(ensureTrailingNewline(out))
	return 0
}

// callBoolFlags are the call flags that take no value; every other --flag
// consumes the following token as its value.
var callBoolFlags = map[string]bool{
	"--stream": true, "-stream": true,
	"--json": true, "-json": true,
	"--help": true, "-h": true,
}

// hoistFlags reorders args so all flags (and their values) precede positionals,
// letting the standard flag parser see every flag regardless of where the
// positional path appears (`call /v1/x --data ...` or `call --data ... /v1/x`).
func hoistFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		// `--flag=value` carries its own value; a bare bool flag takes none.
		// Otherwise the next token is this flag's value.
		if strings.Contains(a, "=") || callBoolFlags[a] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

func printCallUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo call — raw passthrough to any /v1/* gateway endpoint

USAGE:
    modelgo call <path> [flags]

Covers every OpenAI-compatible endpoint, including multimodal ones not wrapped
by chat/models/embeddings: images, audio, anthropic messages, rerank, etc.

FLAGS:
    --method METHOD     HTTP method (default POST with a body, GET without)
    --data JSON         Request body as a literal JSON string
    --data-file FILE    Request body from a file ("-" = stdin)
    --header "K: V"     Extra request header (repeatable)
    --stream            Stream the response body verbatim
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path

EXAMPLES:
    modelgo call /v1/images/generations --data '{"model":"dall-e-3","prompt":"a cat"}'
    modelgo call /v1/messages --data-file anthropic_req.json
    modelgo call /v1/audio/speech --data '{"model":"tts-1","input":"hi","voice":"alloy"}' > out.mp3
    modelgo call /v1/embeddings/multimodal --data-file req.json
    echo '{"model":"gpt-4o","messages":[...]}' | modelgo call /v1/chat/completions --stream`)
}
