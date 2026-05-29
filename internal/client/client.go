// Package client is a thin HTTP client over the orchid server's API.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"orchid/internal/runtime"
	"orchid/internal/session"
)

// ErrNotRunning is returned when no live daemon is recorded.
var ErrNotRunning = errors.New("orchid not running (run: orchid start)")

type Client struct {
	addr string
	hc   *http.Client
}

func New(addr string) *Client {
	return &Client{addr: addr, hc: &http.Client{Timeout: 10 * time.Second}}
}

// FromRuntime builds a client from the recorded daemon, or ErrNotRunning if the
// runtime file is absent or its pid is dead (a stale file is removed).
func FromRuntime() (*Client, error) {
	info, err := runtime.Read()
	if err != nil {
		return nil, ErrNotRunning
	}
	if !runtime.Alive(info.PID) {
		_ = runtime.Remove()
		return nil, ErrNotRunning
	}
	return New(info.Addr), nil
}

func (c *Client) BaseURL() string { return "http://" + c.addr }

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL()+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, ErrNotRunning
	}
	return resp, nil
}

func errFromResp(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s", strings.TrimSpace(string(b)))
}

func (c *Client) List() ([]session.Info, error) {
	resp, err := c.do(http.MethodGet, "/api/sessions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errFromResp(resp)
	}
	var infos []session.Info
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, err
	}
	return infos, nil
}

func (c *Client) Create(agent, cwd string) (session.Info, error) {
	body, _ := json.Marshal(map[string]string{"agent": agent, "cwd": cwd})
	resp, err := c.do(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	if err != nil {
		return session.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return session.Info{}, errFromResp(resp)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return session.Info{}, err
	}
	return info, nil
}

func (c *Client) Kill(id string) error {
	resp, err := c.do(http.MethodDelete, "/api/sessions/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return errFromResp(resp)
	}
	return nil
}

func (c *Client) Restart(id string) (session.Info, error) {
	resp, err := c.do(http.MethodPost, "/api/sessions/"+id+"/restart", nil)
	if err != nil {
		return session.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return session.Info{}, errFromResp(resp)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return session.Info{}, err
	}
	return info, nil
}
