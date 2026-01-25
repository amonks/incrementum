package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
)

func postJSON(ctx context.Context, client *http.Client, baseURL, path string, payload any, dest any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
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

type doRequest struct {
	TodoID string `json:"todo_id"`
}

type doResponse struct {
	JobID string `json:"job_id"`
}

type killRequest struct {
	JobID string `json:"job_id"`
}

type logsRequest struct {
	JobID string `json:"job_id"`
}

type logsResponse struct {
	Events []job.Event `json:"events"`
}

type listResponse struct {
	Jobs []job.Job `json:"jobs"`
}

type listRequest struct {
	Filter job.ListFilter `json:"filter"`
}

type todosListRequest struct {
	Filter todo.ListFilter `json:"filter"`
}

type todosListResponse struct {
	Todos []todo.Todo `json:"todos"`
}

type todosCreateRequest struct {
	Title   string             `json:"title"`
	Options todo.CreateOptions `json:"options"`
}

type todosCreateResponse struct {
	Todo todo.Todo `json:"todo"`
}

type todosUpdateRequest struct {
	IDs     []string           `json:"ids"`
	Options todo.UpdateOptions `json:"options"`
}

type todosUpdateResponse struct {
	Todos []todo.Todo `json:"todos"`
}

type emptyResponse struct{}
