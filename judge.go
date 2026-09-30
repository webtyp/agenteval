//go:build !wasm

package agenteval

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultJudgeURL       = "http://127.0.0.1:8090"
	envJudgeURL           = "AGENTEVAL_JUDGE_URL"
	judgeCompletionPath   = "/completion"
	judgeTokenizePath     = "/tokenize"
	judgeHealthPath       = "/health"
	MinConfidence         = 0.8
	errJudgeUnreachable   = "agenteval: no judge at %s; start it with: llama-server -m ~/Dev/LMmodels/Mapika/decider-4b-GGUF/decider-4b-v2.1-Q4_K_M.gguf --port 8090 -np 1 -c 4096 -ngl 99 (or set AGENTEVAL_JUDGE_URL)"
)

// Judge answers a closed question about a text with a probability for every option.
type Judge interface {
	Decide(state, question string, options []string, kind QuestionKind) (Decision, error)
}

type QuestionKind int

const (
	Choice QuestionKind = iota + 1 // temperature 1.11
	YesNo                          // options must be exactly ["no", "yes"]; temperature 1.56
)

type Decision struct {
	Choice     string    // the most probable option
	Confidence float64   // its probability
	Probs      []float64 // one per option, in the order given
}

type deciderJudge struct {
	url    string
	client *http.Client
}

// NewDeciderJudge creates a new decider-4b judge client.
func NewDeciderJudge(url string) Judge {
	u := url
	if u == "" {
		u = os.Getenv(envJudgeURL)
	}
	if u == "" {
		u = defaultJudgeURL
	}
	return &deciderJudge{
		url:    u,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (j *deciderJudge) available() bool {
	resp, err := j.client.Get(j.url + judgeHealthPath)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type judgeTokenizeReq struct {
	Content    string `json:"content"`
	AddSpecial bool   `json:"add_special"`
}

type judgeTokenizeResp struct {
	Tokens []int `json:"tokens"`
}

type judgeCompletionReq struct {
	Prompt             []int   `json:"prompt"`
	NPredict           int     `json:"n_predict"`
	NProbs             int     `json:"n_probs"`
	Temperature        float64 `json:"temperature"`
	CachePrompt        bool    `json:"cache_prompt"`
	PostSamplingProbs bool    `json:"post_sampling_probs"`
}

type judgeCompletionResp struct {
	CompletionProbabilities []struct {
		TopLogprobs []struct {
			ID      int     `json:"id"`
			Logprob float64 `json:"logprob"`
		} `json:"top_logprobs"`
	} `json:"completion_probabilities"`
}

func (j *deciderJudge) tokenize(text string) ([]int, error) {
	reqBody, _ := json.Marshal(judgeTokenizeReq{
		Content:    text,
		AddSpecial: false,
	})
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", j.url+judgeTokenizePath, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := j.client.Do(httpReq)
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

func (j *deciderJudge) Decide(state, question string, options []string, kind QuestionKind) (Decision, error) {
	numOpts := len(options)
	if numOpts < 1 || numOpts > 10 {
		return Decision{}, fmt.Errorf("agenteval: the judge takes 1 to 10 options, got %d", numOpts)
	}

	part1 := "Context:\n" + state
	part2 := "\n\nQuestion: " + question + "\nOptions:\n"
	for i, opt := range options {
		letter := string(rune('A' + i))
		part2 += fmt.Sprintf("(%s) %s\n", letter, opt)
	}
	part2 += "Answer: ("

	tokens1, err := j.tokenize(part1)
	if err != nil {
		return Decision{}, fmt.Errorf("tokenize part1 failed: %w", err)
	}
	tokens2, err := j.tokenize(part2)
	if err != nil {
		return Decision{}, fmt.Errorf("tokenize part2 failed: %w", err)
	}

	promptIDs := append([]int(nil), tokens1...)
	promptIDs = append(promptIDs, tokens2...)

	// Get letter token IDs
	letterTokenIDs := make([]int, numOpts)
	for i := 0; i < numOpts; i++ {
		letter := string(rune('A' + i))
		toks, err := j.tokenize(letter)
		if err != nil || len(toks) == 0 {
			return Decision{}, fmt.Errorf("tokenize letter %s failed: %w", letter, err)
		}
		letterTokenIDs[i] = toks[0]
	}

	compReq := judgeCompletionReq{
		Prompt:             promptIDs,
		NPredict:           1,
		NProbs:             64,
		Temperature:        0,
		CachePrompt:        false,
		PostSamplingProbs: false,
	}

	bodyBytes, _ := json.Marshal(compReq)
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", j.url+judgeCompletionPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return Decision{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := j.client.Do(httpReq)
	if err != nil {
		return Decision{}, fmt.Errorf("judge completion request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Decision{}, fmt.Errorf("judge completion status: %d", resp.StatusCode)
	}

	var compResp judgeCompletionResp
	if err := json.NewDecoder(resp.Body).Decode(&compResp); err != nil {
		return Decision{}, fmt.Errorf("failed to decode completion response: %w", err)
	}

	if len(compResp.CompletionProbabilities) == 0 {
		return Decision{}, fmt.Errorf("no completion probabilities returned")
	}

	topLogprobs := compResp.CompletionProbabilities[0].TopLogprobs
	logprobMap := make(map[int]float64)
	for _, item := range topLogprobs {
		logprobMap[item.ID] = item.Logprob
	}

	temp := 1.11
	if kind == YesNo {
		temp = 1.56
	}

	zs := make([]float64, numOpts)
	maxZ := -1e18
	for i, tokID := range letterTokenIDs {
		lp, ok := logprobMap[tokID]
		if !ok {
			lp = -1e9
		}
		z := lp / temp
		zs[i] = z
		if z > maxZ {
			maxZ = z
		}
	}

	sumExp := 0.0
	exps := make([]float64, numOpts)
	for i, z := range zs {
		e := math.Exp(z - maxZ)
		exps[i] = e
		sumExp += e
	}

	probs := make([]float64, numOpts)
	bestIdx := 0
	bestProb := -1.0

	for i, e := range exps {
		p := e / sumExp
		probs[i] = p
		if p > bestProb {
			bestProb = p
			bestIdx = i
		}
	}

	return Decision{
		Choice:     options[bestIdx],
		Confidence: bestProb,
		Probs:      probs,
	}, nil
}

func buildJudgeState(a Attempt) string {
	dayOfWeek := time.Weekday(0)
	loc := time.FixedZone("", a.At.UTCOffsetMinutes*60)
	t := time.Date(a.At.Year, time.Month(a.At.Month), a.At.Day, a.At.Hour, a.At.Minute, 0, 0, loc)
	dayOfWeek = t.Weekday()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Today is %s %04d-%02d-%02d %02d:%02d.\n", dayOfWeek.String(), a.At.Year, a.At.Month, a.At.Day, a.At.Hour, a.At.Minute))
	sb.WriteString(fmt.Sprintf("User asked: %s\n", a.Question))
	for _, call := range a.Calls {
		sb.WriteString(fmt.Sprintf("Tool %s returned: %s\n", call.Name, call.Output))
	}
	sb.WriteString(fmt.Sprintf("Assistant answered: %s", a.Answer))
	return sb.String()
}

// Faithful checks if the assistant's answer states anything not supported by the tool result.
func Faithful() Check {
	return faithfulCheck{}
}

type faithfulCheck struct{}

func (c faithfulCheck) Name() string { return "Faithful()" }
func (c faithfulCheck) NeedsJudge() bool { return true }

func (c faithfulCheck) Check(a Attempt, j Judge) Result {
	if j == nil {
		return Result{Verdict: Unsure, Reason: "no judge provided"}
	}
	state := buildJudgeState(a)
	question := "Does the assistant's answer state anything that the tool result does not support?"
	options := []string{
		"no, everything it says is supported by the tool result",
		"yes, it states something the tool result does not support",
	}

	dec, err := j.Decide(state, question, options, Choice)
	if err != nil {
		return Result{Verdict: Unsure, Reason: fmt.Sprintf("judge error: %v", err)}
	}

	if dec.Confidence < MinConfidence {
		return Result{
			Verdict: Unsure,
			Reason:  fmt.Sprintf("the judge chose %q with confidence %.2f (below 0.80)", dec.Choice, dec.Confidence),
		}
	}

	if dec.Choice == options[0] {
		return Result{Verdict: Pass, Reason: "the assistant answer is faithful to tool results"}
	}
	return Result{Verdict: Fail, Reason: "the assistant answer states something not supported by tool results"}
}

// AnswersTheQuestion checks if the assistant's answer actually answers the user's question.
func AnswersTheQuestion() Check {
	return answersTheQuestionCheck{}
}

type answersTheQuestionCheck struct{}

func (c answersTheQuestionCheck) Name() string { return "AnswersTheQuestion()" }
func (c answersTheQuestionCheck) NeedsJudge() bool { return true }

func (c answersTheQuestionCheck) Check(a Attempt, j Judge) Result {
	if j == nil {
		return Result{Verdict: Unsure, Reason: "no judge provided"}
	}
	state := buildJudgeState(a)
	question := "Does the assistant's answer actually answer the user's question?"
	options := []string{"yes", "no"}

	dec, err := j.Decide(state, question, options, Choice)
	if err != nil {
		return Result{Verdict: Unsure, Reason: fmt.Sprintf("judge error: %v", err)}
	}

	if dec.Confidence < MinConfidence {
		return Result{
			Verdict: Unsure,
			Reason:  fmt.Sprintf("the judge chose %q with confidence %.2f (below 0.80)", dec.Choice, dec.Confidence),
		}
	}

	if dec.Choice == "yes" {
		return Result{Verdict: Pass, Reason: "the assistant answered the question"}
	}
	return Result{Verdict: Fail, Reason: "the assistant did not answer the question"}
}
