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
	"strings"
	"time"

	"webtyp.com/agentcontext"
	"webtyp.com/context"
	"webtyp.com/llm"
)

const (
	defaultModelURL           = "http://127.0.0.1:8080"
	envModelURL               = "AGENTEVAL_MODEL_URL"
	llamaChatPath             = "/v1/chat/completions"
	llamaTokenizePath         = "/tokenize"
	llamaPropsPath            = "/props"
	llamaApplyTemplatePath    = "/apply-template"
	llamaCompletionPath       = "/completion"
	DefaultOutputTokens       = 512
	errModelServerUnreachable = "agenteval: no model server at %s; start one with: llama-server -m <model.gguf> --port 8080 --jinja -c 4096 --reasoning-budget 0 --temp 0.7 --top-p 0.8 --top-k 20 --min-p 0 (or set AGENTEVAL_MODEL_URL)"
)

type modelServer struct {
	url    string
	client *http.Client
	seed   int // the sampling seed of one attempt: attempts differ, and a rerun repeats them
}

// forAttempt is the same server sampled with the seed of attempt i. The sampling settings
// (temperature, top-p, top-k) are the ones llama-server was started with, so they belong to
// the model, not to this library.
func (m *modelServer) forAttempt(i int) *modelServer {
	return &modelServer{url: m.url, client: m.client, seed: i}
}

func newModelServer(overrideURL string) *modelServer {
	u := overrideURL
	if u == "" {
		u = os.Getenv(envModelURL)
	}
	if u == "" {
		u = defaultModelURL
	}
	return &modelServer{
		url:    u,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

type llamaMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []llamaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type llamaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type llamaToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type llamaChatRequest struct {
	Messages  []llamaMessage `json:"messages"`
	Tools     []llamaToolDef `json:"tools,omitempty"`
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

type llamaTokenizeRequest struct {
	Content string `json:"content"`
}

type llamaTokenizeResponse struct {
	Tokens []any `json:"tokens"`
}

type llamaPropsResponse struct {
	DefaultGenerationSettings struct {
		NCtx int `json:"n_ctx"`
	} `json:"default_generation_settings"`
}

type llamaApplyTemplateRequest struct {
	Messages []llamaMessage `json:"messages"`
}

type llamaApplyTemplateResponse struct {
	Prompt string `json:"prompt"`
}

type llamaCompletionReq struct {
	Prompt      string  `json:"prompt"`
	NPredict    int     `json:"n_predict"`
	NProbs      int     `json:"n_probs"`
	Temperature float64 `json:"temperature"`
	CachePrompt bool    `json:"cache_prompt"`
}

type llamaCompletionResp struct {
	CompletionProbabilities []struct {
		TopLogprobs []struct {
			ID      int     `json:"id"`
			Logprob float64 `json:"logprob"`
		} `json:"top_logprobs"`
	} `json:"completion_probabilities"`
}

func (m *modelServer) budget() (agentcontext.Budget, error) {
	resp, err := m.client.Get(m.url + llamaPropsPath)
	if err != nil {
		return agentcontext.Budget{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return agentcontext.Budget{}, fmt.Errorf("props status: %d", resp.StatusCode)
	}

	var props llamaPropsResponse
	if err := json.NewDecoder(resp.Body).Decode(&props); err != nil {
		return agentcontext.Budget{}, err
	}

	nCtx := props.DefaultGenerationSettings.NCtx
	if nCtx <= 0 {
		nCtx = 4096
	}

	return agentcontext.Budget{
		ContextTokens: nCtx,
		OutputTokens:  DefaultOutputTokens,
	}, nil
}

func (m *modelServer) Generate(ctx *context.Context, req llm.Request) (llm.Response, error) {
	var messages []llamaMessage

	if req.System != "" {
		messages = append(messages, llamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, msg := range req.Messages {
		lm := llamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		if msg.Role == llm.RoleTool {
			lm.ToolCallID = msg.ToolCallID
		}
		if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
			var tcs []llamaToolCall
			for _, tc := range msg.ToolCalls {
				tcs = append(tcs, llamaToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
			lm.ToolCalls = tcs
		}
		messages = append(messages, lm)
	}

	var tools []llamaToolDef
	for _, t := range req.Tools {
		tools = append(tools, llamaToolDef{
			Type: "function",
			Function: struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			}{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(t.InputSchema),
			},
		})
	}

	bodyObj := llamaChatRequest{
		Messages:  messages,
		Tools:     tools,
		MaxTokens: req.MaxOutputTokens,
		Stream:    false,
		Seed:      m.seed,
	}

	bodyBytes, _ := json.Marshal(bodyObj)
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", m.url+llamaChatPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return llm.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return llm.Response{}, fmt.Errorf("llama-server request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return llm.Response{}, fmt.Errorf("llama-server error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp llamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return llm.Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return llm.Response{}, fmt.Errorf("no choices returned")
	}

	choice := chatResp.Choices[0]

	llmResp := llm.Response{
		Text:  choice.Message.Content,
		Usage: llm.Usage{InputTokens: chatResp.Usage.PromptTokens, OutputTokens: chatResp.Usage.CompletionTokens},
	}

	switch choice.FinishReason {
	case "tool_calls":
		llmResp.StopReason = llm.StopToolUse
	case "length":
		llmResp.StopReason = llm.StopMaxTokens
	default:
		if len(choice.Message.ToolCalls) > 0 {
			llmResp.StopReason = llm.StopToolUse
		} else {
			llmResp.StopReason = llm.StopEndTurn
		}
	}

	for _, tc := range choice.Message.ToolCalls {
		llmResp.ToolCalls = append(llmResp.ToolCalls, llm.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	return llmResp, nil
}

func (m *modelServer) CountTokens(text string) int {
	bodyBytes, _ := json.Marshal(llamaTokenizeRequest{Content: text})
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", m.url+llamaTokenizePath, bytes.NewReader(bodyBytes))
	if err != nil {
		return len(text) / 4
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return len(text) / 4
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return len(text) / 4
	}

	var tokResp llamaTokenizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil {
		return len(text) / 4
	}

	return len(tokResp.Tokens)
}

func (m *modelServer) tokenize(text string) ([]int, error) {
	reqBody, _ := json.Marshal(judgeTokenizeReq{
		Content:    text,
		AddSpecial: false,
	})
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", m.url+llamaTokenizePath, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tokenize status: %d", resp.StatusCode)
	}

	var tokResp judgeTokenizeResp
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil {
		return nil, err
	}
	return tokResp.Tokens, nil
}

func formatCriticContent(q llm.Question) string {
	var sb strings.Builder
	sb.WriteString("Context:\n")
	sb.WriteString(q.Context)
	sb.WriteString("\n\nQuestion: ")
	sb.WriteString(q.Text)
	sb.WriteString("\nOptions:\n")
	for i, opt := range q.Options {
		sb.WriteString(fmt.Sprintf("(%c) %s\n", 'A'+i, opt))
	}
	sb.WriteString("Answer with the letter of one option.")
	return sb.String()
}

func (m *modelServer) Decide(ctx *context.Context, q llm.Question) (llm.Decision, error) {
	numOpts := len(q.Options)
	if numOpts < 1 || numOpts > 10 {
		return llm.Decision{}, fmt.Errorf("agenteval: critic takes 1 to 10 options, got %d", numOpts)
	}

	content := formatCriticContent(q)
	applyReq := llamaApplyTemplateRequest{
		Messages: []llamaMessage{{Role: "user", Content: content}},
	}
	bodyBytes, _ := json.Marshal(applyReq)

	reqCtx := stdcontext.Background()

	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", m.url+llamaApplyTemplatePath, bytes.NewReader(bodyBytes))
	if err != nil {
		return llm.Decision{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return llm.Decision{}, fmt.Errorf("apply-template request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llm.Decision{}, fmt.Errorf("apply-template status: %d", resp.StatusCode)
	}

	var applyResp llamaApplyTemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&applyResp); err != nil {
		return llm.Decision{}, fmt.Errorf("failed to decode apply-template response: %w", err)
	}

	letterTokenIDs := make([]int, numOpts)
	for i := 0; i < numOpts; i++ {
		letter := string(rune('A' + i))
		toks, err := m.tokenize(letter)
		if err != nil || len(toks) == 0 {
			return llm.Decision{}, fmt.Errorf("tokenize letter %s failed: %w", letter, err)
		}
		letterTokenIDs[i] = toks[0]
	}

	compReq := llamaCompletionReq{
		Prompt:      applyResp.Prompt + "(",
		NPredict:    1,
		NProbs:      64,
		Temperature: 0,
		CachePrompt: false,
	}

	compBytes, _ := json.Marshal(compReq)
	compHttpReq, err := http.NewRequestWithContext(reqCtx, "POST", m.url+llamaCompletionPath, bytes.NewReader(compBytes))
	if err != nil {
		return llm.Decision{}, err
	}
	compHttpReq.Header.Set("Content-Type", "application/json")

	compResp, err := m.client.Do(compHttpReq)
	if err != nil {
		return llm.Decision{}, fmt.Errorf("critic completion request failed: %w", err)
	}
	defer compResp.Body.Close()

	if compResp.StatusCode != http.StatusOK {
		return llm.Decision{}, fmt.Errorf("critic completion status: %d", compResp.StatusCode)
	}

	var compResult llamaCompletionResp
	if err := json.NewDecoder(compResp.Body).Decode(&compResult); err != nil {
		return llm.Decision{}, fmt.Errorf("failed to decode completion response: %w", err)
	}

	if len(compResult.CompletionProbabilities) == 0 {
		return llm.Decision{}, fmt.Errorf("no completion probabilities returned")
	}

	topLogprobs := compResult.CompletionProbabilities[0].TopLogprobs
	logprobMap := make(map[int]float64)
	for _, item := range topLogprobs {
		logprobMap[item.ID] = item.Logprob
	}

	bestIdx, bestProb, probs := computeOptionDecision(logprobMap, letterTokenIDs, 1.0)

	return llm.Decision{
		Choice:     bestIdx,
		Confidence: bestProb,
		Probs:      probs,
	}, nil
}
