// Package modelcmd implements the modelgo model data-plane commands —
// `chat`, `models`, `embeddings`, and the raw `call` passthrough — which call
// the gateway's OpenAI-compatible /v1/* endpoints using a model API key
// (resolved via --api-key, MODELGO_API_KEY, or `modelgo key set`).
package modelcmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/modelapi"
)

// commonFlags holds the flags shared by every model command.
type commonFlags struct {
	apiKey     *string
	env        *string
	configPath *string
	jsonOut    *bool
}

func registerCommon(fs *flag.FlagSet) commonFlags {
	return commonFlags{
		apiKey:     fs.String("api-key", "", "model API key (overrides MODELGO_API_KEY and stored key)"),
		env:        fs.String("env", "", "env to call (default: active env from config)"),
		configPath: fs.String("config", "", "config file path (default ~/.modelgo/config.json)"),
		jsonOut:    fs.Bool("json", false, "write the raw JSON response"),
	}
}

func (cf commonFlags) client() (*modelapi.Client, error) {
	return modelapi.New(modelapi.Params{
		APIKeyFlag: *cf.apiKey,
		EnvFlag:    *cf.env,
		ConfigPath: *cf.configPath,
	})
}

// readPrompt joins positional args into a prompt. If there are none, or the sole
// arg is "-", it reads the prompt from stdin. Returns an error when nothing is
// available so commands fail loudly instead of sending an empty request.
func readPrompt(args []string, stdin io.Reader) (string, error) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return "", fmt.Errorf("no prompt provided (pass it as an argument or via stdin)")
		}
		return s, nil
	}
	return strings.Join(args, " "), nil
}

// readData resolves a request body from --data, --data-file ("-" = stdin), or
// stdin when both are empty.
func readData(data, dataFile string, stdin io.Reader) ([]byte, error) {
	switch {
	case data != "":
		return []byte(data), nil
	case dataFile == "-":
		return io.ReadAll(stdin)
	case dataFile != "":
		return os.ReadFile(dataFile)
	default:
		return io.ReadAll(stdin)
	}
}

func fail(stderr io.Writer, prefix string, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
	return 1
}
