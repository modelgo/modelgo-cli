package modelcmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// imageList collects repeated --image flags.
type imageList []string

func (l *imageList) String() string { return strings.Join(*l, ",") }
func (l *imageList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// Chat runs `modelgo chat`: a chat-completions convenience wrapper over
// POST /v1/chat/completions. The prompt comes from positional args or stdin.
func Chat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printChatUsage(stdout)
			return 0
		}
	}

	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := registerCommon(fs)
	model := fs.String("model", "", "model id (required)")
	system := fs.String("system", "", "optional system prompt")
	stream := fs.Bool("stream", false, "stream the response token-by-token")
	maxTokens := fs.Int("max-tokens", 0, "max output tokens (0 = provider default)")
	temperature := fs.Float64("temperature", -1, "sampling temperature (negative = provider default)")
	var images imageList
	fs.Var(&images, "image", "image URL or local file path for vision models (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *model == "" {
		fmt.Fprintln(stderr, "chat: --model is required")
		return 2
	}

	prompt, err := readPrompt(fs.Args(), stdin)
	if err != nil {
		return fail(stderr, "chat", err)
	}

	userContent, err := buildUserContent(prompt, images)
	if err != nil {
		return fail(stderr, "chat", err)
	}

	messages := []map[string]any{}
	if *system != "" {
		messages = append(messages, map[string]any{"role": "system", "content": *system})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userContent})

	body := map[string]any{"model": *model, "messages": messages}
	if *stream {
		body["stream"] = true
	}
	if *maxTokens > 0 {
		body["max_tokens"] = *maxTokens
	}
	if *temperature >= 0 {
		body["temperature"] = *temperature
	}
	payload, _ := json.Marshal(body)

	client, err := cf.client()
	if err != nil {
		return fail(stderr, "chat", err)
	}

	ctx := context.Background()
	resp, err := client.Do(ctx, http.MethodPost, "/v1/chat/completions", strings.NewReader(string(payload)), nil)
	if err != nil {
		return fail(stderr, "chat", err)
	}
	defer resp.Body.Close()

	if *stream {
		return streamChat(resp.Body, *cf.jsonOut, stdout, stderr)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fail(stderr, "chat", err)
	}
	if *cf.jsonOut {
		stdout.Write(ensureTrailingNewline(data))
		return 0
	}
	content, err := extractMessageContent(data)
	if err != nil {
		return fail(stderr, "chat", err)
	}
	fmt.Fprintln(stdout, content)
	return 0
}

// buildUserContent returns a plain string when no images are attached, or an
// OpenAI multimodal content array (text + image_url parts) otherwise.
func buildUserContent(prompt string, images imageList) (any, error) {
	if len(images) == 0 {
		return prompt, nil
	}
	parts := []map[string]any{{"type": "text", "text": prompt}}
	for _, img := range images {
		url, err := resolveImageURL(img)
		if err != nil {
			return nil, err
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		})
	}
	return parts, nil
}

// resolveImageURL passes through http(s):// and data: URLs, and reads a local
// file into a base64 data URL otherwise.
func resolveImageURL(img string) (string, error) {
	if strings.HasPrefix(img, "http://") || strings.HasPrefix(img, "https://") || strings.HasPrefix(img, "data:") {
		return img, nil
	}
	b, err := os.ReadFile(img)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", img, err)
	}
	mt := mime.TypeByExtension(filepath.Ext(img))
	if mt == "" {
		mt = http.DetectContentType(b)
	}
	return "data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// streamChat reads an SSE chat-completions stream, printing assistant deltas to
// stdout as they arrive. With jsonOut it forwards each raw data frame verbatim.
func streamChat(r io.Reader, jsonOut bool, stdout, stderr io.Writer) int {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	wrote := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		if jsonOut {
			fmt.Fprintln(stdout, payload)
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // tolerate keep-alive / comment frames
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				fmt.Fprint(stdout, c.Delta.Content)
				wrote = true
			}
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(stderr, "\nchat: stream error: %v\n", err)
		return 1
	}
	if wrote && !jsonOut {
		fmt.Fprintln(stdout)
	}
	return 0
}

// extractMessageContent pulls choices[0].message.content out of a non-stream
// chat-completions response.
func extractMessageContent(data []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}

func ensureTrailingNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	return append(b, '\n')
}

func printChatUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo chat — call a chat model (OpenAI-compatible /v1/chat/completions)

USAGE:
    modelgo chat --model MODEL [flags] [PROMPT...]
    echo "PROMPT" | modelgo chat --model MODEL

FLAGS:
    --model MODEL       Model id (required)
    --system TEXT       System prompt
    --stream            Stream the response token-by-token
    --image SRC         Image URL or local file for vision models (repeatable)
    --max-tokens N      Max output tokens
    --temperature T     Sampling temperature (0–2)
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path
    --json              Print the raw JSON response (NDJSON frames when --stream)

EXAMPLES:
    modelgo chat --model gpt-4o "Write a haiku about Go"
    modelgo chat --model gpt-4o --stream "Explain channels"
    modelgo chat --model gpt-4o --image photo.png "What is in this image?"
    cat prompt.txt | modelgo chat --model claude-opus-4-8`)
}
