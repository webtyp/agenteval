//go:build !wasm

package agenteval

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"webtyp.com/context"
	"webtyp.com/llm"
	"webtyp.com/qwen"
)

const (
	defaultDeciderURL = "http://127.0.0.1:8080"
	envDeciderURL     = "AGENTEVAL_DECIDER_URL"
	defaultWriterURL  = "http://127.0.0.1:8081"
	envWriterURL      = "AGENTEVAL_WRITER_URL"
	llamaChatPath     = "/v1/chat/completions"
	llamaTokenizePath = "/tokenize"
	llamaHealthPath   = "/health"
	// DecideTemperature is decider-0.8b's decision temperature, the one the browser uses
	// (qwen.Config.DecideTemperature).
	DecideTemperature     = 1.03
	errDeciderUnreachable = "agenteval: no decision model at %s; start it with: llama-server -m ~/Dev/LMmodels/mradermacher/decider-0.8b-GGUF/decider-0.8b.Q8_0.gguf --port 8080 -np 1 -c 4096 (or set AGENTEVAL_DECIDER_URL)"
	errWriterUnreachable  = "agenteval: no writer at %s; start it with: llama-server -m ~/Dev/LMmodels/LiquidAI/LFM2.5-350M-GGUF/LFM2.5-350M-Q8_0.gguf --port 8081 --jinja -np 1 -c 4096 --temp 0.3 (or set AGENTEVAL_WRITER_URL)"
	errDecideOptions      = "agenteval: a decision takes 2 to 10 options, got %d"
)

// serverURL returns override, else the environment variable, else def.
func serverURL(override, env, def string) string {
	if override != "" {
		return override
	}
	if u := os.Getenv(env); u != "" {
		return u
	}
	return def
}

func newHTTPClient() *http.Client { return &http.Client{Timeout: 120 * time.Second} }

// healthy reports whether a llama-server answers its /health.
func healthy(client *http.Client, url string) bool {
	resp, err := client.Get(url + llamaHealthPath)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// deciderServer is decider-0.8b on llama-server, asked exactly as webtyp.com/qwen asks it in the
// browser: the pieces of qwen.DecidePrompt, each tokenized on its own, and the option letters read
// from the next token's probabilities.
type deciderServer struct {
	url    string
	client *http.Client
	tok    *llamaTokenizer
}

func newDeciderServer(override string) *deciderServer {
	u := serverURL(override, envDeciderURL, defaultDeciderURL)
	client := newHTTPClient()
	return &deciderServer{url: u, client: client, tok: &llamaTokenizer{url: u, client: client}}
}

func (d *deciderServer) available() bool { return healthy(d.client, d.url) }

// Decide answers a closed question by the probability of each option's letter.
func (d *deciderServer) Decide(ctx *context.Context, q llm.Question) (llm.Decision, error) {
	n := len(q.Options)
	if n < 2 || n > 10 {
		return llm.Decision{}, fmt.Errorf(errDecideOptions, n)
	}
	var ids []int
	for _, piece := range qwen.DecidePrompt(q) {
		toks, err := d.tok.tokenize(piece)
		if err != nil {
			return llm.Decision{}, err
		}
		ids = append(ids, toks...)
	}
	letters, err := d.tok.letterIDs(n)
	if err != nil {
		return llm.Decision{}, err
	}
	logprobs, err := completionLogprobs(d.client, d.url, judgeCompletionReq{
		Prompt: ids, NPredict: 1, NProbs: 100, Temperature: 0, CachePrompt: false,
	})
	if err != nil {
		return llm.Decision{}, err
	}
	choice, confidence, probs := computeOptionDecision(logprobs, letters, DecideTemperature)
	return llm.Decision{Choice: choice, Confidence: confidence, Probs: probs}, nil
}

// CountTokens counts with the decider's tokenizer; a failed request falls back to len/4.
func (d *deciderServer) CountTokens(text string) int {
	toks, err := d.tok.tokenize(text)
	if err != nil {
		return len(text) / 4
	}
	return len(toks)
}

// completionLogprobs posts one /completion and returns the first token's top logprobs by id.
func completionLogprobs(client *http.Client, url string, req judgeCompletionReq) (map[int]float64, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", url+judgeCompletionPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("completion status: %d", resp.StatusCode)
	}
	var out judgeCompletionResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode completion response: %w", err)
	}
	if len(out.CompletionProbabilities) == 0 {
		return nil, fmt.Errorf("no completion probabilities returned")
	}
	lp := make(map[int]float64)
	for _, item := range out.CompletionProbabilities[0].TopLogprobs {
		lp[item.ID] = item.Logprob
	}
	return lp, nil
}

// writerServer is LFM2.5-350M on llama-server: it phrases answers from data and never sees tools.
type writerServer struct {
	url    string
	client *http.Client
	seed   int
}

func newWriterServer(override string) *writerServer {
	return &writerServer{url: serverURL(override, envWriterURL, defaultWriterURL), client: newHTTPClient()}
}

func (w *writerServer) available() bool { return healthy(w.client, w.url) }

// forAttempt returns the writer with attempt i's seed, so attempts vary and a rerun repeats them.
func (w *writerServer) forAttempt(i int) *writerServer {
	return &writerServer{url: w.url, client: w.client, seed: i}
}

type llamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
}

type llamaChatRequest struct {
	Messages  []llamaMessage `json:"messages"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Stream    bool           `json:"stream"`
	Seed      int            `json:"seed"`
}

type llamaChatResponse struct {
	Choices []struct {
		Message      llamaMessage `json:"message"`
		FinishReason string       `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Generate asks the writer through the chat completions endpoint. Every llm.RoleSystem message is
// folded into one system block at the start: chat templates accept only that.
func (w *writerServer) Generate(ctx *context.Context, req llm.Request) (llm.Response, error) {
	var messages []llamaMessage
	system := req.System
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem && msg.Content != "" {
			if system != "" {
				system += "\n\n"
			}
			system += msg.Content
		}
	}
	if system != "" {
		messages = append(messages, llamaMessage{Role: "system", Content: system})
	}
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem {
			continue
		}
		messages = append(messages, llamaMessage{Role: string(msg.Role), Content: msg.Content})
	}

	body, _ := json.Marshal(llamaChatRequest{Messages: messages, MaxTokens: req.MaxOutputTokens, Seed: w.seed})
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", w.url+llamaChatPath, bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(httpReq)
	if err != nil {
		return llm.Response{}, fmt.Errorf("llama-server request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return llm.Response{}, fmt.Errorf("llama-server error %d: %s", resp.StatusCode, string(b))
	}
	var chat llamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chat); err != nil {
		return llm.Response{}, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(chat.Choices) == 0 {
		return llm.Response{}, fmt.Errorf("no choices returned")
	}
	choice := chat.Choices[0]
	out := llm.Response{
		Text:       choice.Message.Content,
		StopReason: llm.StopEndTurn,
		Usage:      llm.Usage{InputTokens: chat.Usage.PromptTokens, OutputTokens: chat.Usage.CompletionTokens},
	}
	if choice.FinishReason == "length" {
		out.StopReason = llm.StopMaxTokens
	}
	return out, nil
}
