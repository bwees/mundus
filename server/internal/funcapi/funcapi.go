// Package funcapi drives control_center's local Thing-Model funcRequest HTTP API.
//
// No key is shipped or extracted from the vendor binary: control_center generates
// a fresh random token+port at runtime when the local service is enabled, and we
// read them back over its own local terminal. Auth per request is
// md5hex(body + token).
package funcapi

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwees/mundus/server/internal/robot"
)

// Local holds the runtime-acquired endpoint for the funcRequest API. control_center
// generates a fresh port+token each time the local service is enabled and recycles
// them when it restarts, so the endpoint is re-acquired on demand via the terminal.
type Local struct {
	BaseURL string
	Token   string
	Auth    bool
	http    *http.Client
	seq     uint64

	term *robot.Client
	host string
	mu   sync.Mutex
}

type localConnPrint struct {
	IsEnableAuth bool   `json:"is_enable_auth"`
	Port         int    `json:"port"`
	Token        string `json:"token"`
}

// Acquire enables the local HTTP service via the terminal and reads its
// runtime-generated port + token. host is the address to reach the HTTP server
// on (e.g. "127.0.0.1" when running on the device).
func Acquire(term *robot.Client, host string) (*Local, error) {
	if _, err := term.Exec("/cc/iot/local_conn/service_enable"); err != nil {
		return nil, fmt.Errorf("service_enable: %w", err)
	}
	out, err := term.Exec("/cc/iot/local_conn/print")
	if err != nil {
		return nil, fmt.Errorf("local_conn print: %w", err)
	}
	var p localConnPrint
	if !decodeJSON(out, &p) {
		return nil, fmt.Errorf("parse local_conn print: %q", out)
	}
	if p.Port == 0 {
		return nil, fmt.Errorf("local service reported port 0 (not enabled)")
	}
	return &Local{
		BaseURL: fmt.Sprintf("http://%s:%d", host, p.Port),
		Token:   p.Token,
		Auth:    p.IsEnableAuth,
		http:    &http.Client{Timeout: 15 * time.Second},
		term:    term,
		host:    host,
	}, nil
}

// refresh re-enables the local service and reloads the port+token, recovering
// from a control_center restart that recycled the endpoint.
func (l *Local) refresh() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.term == nil {
		return fmt.Errorf("no terminal for re-acquire")
	}
	if _, err := l.term.Exec("/cc/iot/local_conn/service_enable"); err != nil {
		return err
	}
	out, err := l.term.Exec("/cc/iot/local_conn/print")
	if err != nil {
		return err
	}
	var p localConnPrint
	if !decodeJSON(out, &p) || p.Port == 0 {
		return fmt.Errorf("re-acquire parse: %q", out)
	}
	l.BaseURL = fmt.Sprintf("http://%s:%d", l.host, p.Port)
	l.Token = p.Token
	l.Auth = p.IsEnableAuth
	return nil
}

func (l *Local) endpoint() (base, token string, auth bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.BaseURL, l.Token, l.Auth
}

type reqEnvelope struct {
	Code    int     `json:"code"`
	Payload payload `json:"payload"`
}

type payload struct {
	FunctionID int            `json:"functionID"`
	RequestID  string         `json:"requestID"`
	Params     map[string]any `json:"params"`
}

// FuncResponse is the decoded funcResponse payload.
type FuncResponse struct {
	Code    int `json:"code"`
	Payload struct {
		FunctionID int            `json:"functionID"`
		RequestID  string         `json:"requestID"`
		Params     map[string]any `json:"params"`
	} `json:"payload"`
}

// Func sends a Thing-Model funcRequest and returns the decoded response.
func (l *Local) Func(functionID int, params map[string]any) (*FuncResponse, error) {
	if params == nil {
		params = map[string]any{}
	}
	reqID := strconv.FormatUint(atomic.AddUint64(&l.seq, 1), 10)
	body, err := json.Marshal(reqEnvelope{Code: 3,
		Payload: payload{FunctionID: functionID, RequestID: reqID, Params: params}})
	if err != nil {
		return nil, err
	}
	fr, err := l.try(functionID, body)
	if err != nil {
		// The endpoint may be stale (control_center recycled port/token). Re-acquire once and retry.
		if rerr := l.refresh(); rerr != nil {
			return nil, err
		}
		return l.try(functionID, body)
	}
	return fr, nil
}

func (l *Local) try(functionID int, body []byte) (*FuncResponse, error) {
	base, token, auth := l.endpoint()
	req, err := http.NewRequest(http.MethodPost, base+"/thing_model/func_request", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if auth {
		req.Header.Set("Auth", sign(body, token))
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("funcRequest %d: http %d: %s", functionID, resp.StatusCode, rb)
	}
	var fr FuncResponse
	if err := json.Unmarshal(rb, &fr); err != nil {
		return nil, fmt.Errorf("decode response: %w (%s)", err, rb)
	}
	return &fr, nil
}

func sign(body []byte, token string) string {
	h := md5.New()
	h.Write(body)
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func decodeJSON(out string, v any) bool {
	start := strings.IndexByte(out, '{')
	if start < 0 {
		return false
	}
	dec := json.NewDecoder(strings.NewReader(out[start:]))
	return dec.Decode(v) == nil
}
