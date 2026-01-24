package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/amonks/incrementum/job"
)

// Client calls swarm RPCs.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a client for the given address or URL.
func NewClient(addr string) *Client {
	baseURL := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{baseURL: baseURL, client: &http.Client{}}
}

// Do starts a job for a todo and returns the job ID.
func (c *Client) Do(ctx context.Context, todoID string) (string, error) {
	var response doResponse
	if err := c.post(ctx, "/do", doRequest{TodoID: todoID}, &response); err != nil {
		return "", err
	}
	return response.JobID, nil
}

// Kill interrupts a running job.
func (c *Client) Kill(ctx context.Context, jobID string) error {
	return c.post(ctx, "/kill", killRequest{JobID: jobID}, &emptyResponse{})
}

// Logs returns all events recorded so far for a job.
func (c *Client) Logs(ctx context.Context, jobID string) ([]job.Event, error) {
	var response logsResponse
	if err := c.post(ctx, "/logs", logsRequest{JobID: jobID}, &response); err != nil {
		return nil, err
	}
	return response.Events, nil
}

// List returns jobs from the swarm server.
func (c *Client) List(ctx context.Context) ([]job.Job, error) {
	var response listResponse
	if err := c.post(ctx, "/list", emptyResponse{}, &response); err != nil {
		return nil, err
	}
	return response.Jobs, nil
}

// Tail streams events for a job.
func (c *Client) Tail(ctx context.Context, jobID string) (<-chan job.Event, <-chan error) {
	events := make(chan job.Event, 16)
	errCh := make(chan error, 1)

	go func() {
		defer close(events)
		payload, err := json.Marshal(tailRequest{JobID: jobID})
		if err != nil {
			errCh <- err
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tail", bytes.NewReader(payload))
		if err != nil {
			errCh <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errCh <- readErrorResponse(resp)
			return
		}
		decoder := json.NewDecoder(resp.Body)
		for {
			var event job.Event
			if err := decoder.Decode(&event); err != nil {
				if ctx.Err() != nil {
					errCh <- nil
					return
				}
				if errors.Is(err, io.EOF) {
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
			events <- event
		}
	}()

	return events, errCh
}

func (c *Client) post(ctx context.Context, path string, payload any, dest any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readErrorResponse(resp)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

func readErrorResponse(resp *http.Response) error {
	var payload map[string]string
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err == nil {
		if message, ok := payload["error"]; ok {
			return fmt.Errorf("swarm error: %s", message)
		}
	}
	return fmt.Errorf("swarm error: %s", resp.Status)
}
