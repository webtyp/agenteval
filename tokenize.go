//go:build !wasm

package agenteval

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

const optionLetters = "ABCDEFGHIJ"

// llamaTokenizer tokenizes through a llama-server's /tokenize, and keeps the token ids of the
// option letters A..J after the first time they are needed. The judge and the critic both read
// their answer from those letters.
type llamaTokenizer struct {
	url    string
	client *http.Client

	mu      sync.Mutex
	letters []int
}

func (t *llamaTokenizer) tokenize(text string) ([]int, error) {
	reqBody, _ := json.Marshal(judgeTokenizeReq{Content: text, AddSpecial: false})
	httpReq, err := http.NewRequestWithContext(stdcontext.Background(), "POST", t.url+judgeTokenizePath, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(httpReq)
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

// letterIDs returns the token ids of the first n option letters.
func (t *llamaTokenizer) letterIDs(n int) ([]int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.letters == nil {
		ids := make([]int, len(optionLetters))
		for i := range optionLetters {
			toks, err := t.tokenize(optionLetters[i : i+1])
			if err != nil || len(toks) != 1 {
				return nil, fmt.Errorf("agenteval: tokenizing the option letter %q failed: %v", optionLetters[i:i+1], err)
			}
			ids[i] = toks[0]
		}
		t.letters = ids
	}
	return t.letters[:n], nil
}
