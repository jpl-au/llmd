// hook.go handles POST /hook requests from agent hook systems.
//
// Agent platforms (Claude Code, Gemini CLI, etc.) fire hooks on
// lifecycle events like session start, tool completion, and task
// finish. Each platform sends a different JSON shape. The handler
// identifies the platform from the Author header, delegates parsing
// to the platform's ParseHook method, and routes the normalised
// event to the appropriate SDK operations.

package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/assets/platform"
	"github.com/jpl-au/llmd/internal/telemetry"
	"github.com/jpl-au/llmd/sdk"
)

var (
	errMissingAuthor = errors.New("Author header is required")
	errEmptyPayload  = errors.New("request body is empty")
)

// hookHandler returns an http.HandlerFunc for POST /hook.
func (s *Server) hookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		author := r.Header.Get("Author")
		if author == "" {
			author = s.author
		}
		if author == "" {
			writeError(w, http.StatusBadRequest, errMissingAuthor)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		defer r.Body.Close()

		if len(body) == 0 {
			writeError(w, http.StatusBadRequest, errEmptyPayload)
			return
		}

		plat := assets.Agent.Platform(author)
		event, err := plat.ParseHook(body)
		if err != nil {
			slog.Warn("parsing hook payload", "author", author, "error", err)
			writeError(w, http.StatusBadRequest, err)
			return
		}

		telemetry.Emit(telemetry.Entry{
			Source:  "hook",
			Event:   event.Event,
			Author:  author,
			Success: true,
		})

		slog.Debug("hook received", "author", author, "event", event.Event, "task", event.TaskID)

		result := s.routeHook(r, author, event)
		writeJSON(w, http.StatusOK, result)
	}
}

// hookResult is the JSON response returned to the hook caller.
// Platforms like Claude Code parse the response body for control
// decisions.
type hookResult struct {
	OK      bool     `json:"ok"`
	Event   string   `json:"event"`
	Actions []string `json:"actions,omitempty"`
	Content string   `json:"content,omitempty"`
}

// routeHook maps a normalised HookEvent to SDK operations based on
// the event type. Returns a summary of actions taken.
func (s *Server) routeHook(r *http.Request, author string, event *platform.HookEvent) hookResult {
	result := hookResult{OK: true, Event: event.Event}

	switch event.Event {
	case "session.start":
		// Return pending queue messages so the agent can act on them.
		resp, err := sdk.Dispatch(r.Context(), "queue", []string{"ls"}, author, nil, "")
		if err != nil {
			slog.Debug("hook: fetching queue on session.start", "error", err)
			break
		}
		result.Content = responseText(resp)
		result.Actions = append(result.Actions, "queue.ls")

	case "session.end":
		// Notify via queue that the session ended.
		content := event.Content
		if content == "" {
			content = "Session ended"
		}
		args := []string{"send", content}
		if _, err := sdk.Dispatch(r.Context(), "queue", args, author, nil, ""); err != nil {
			slog.Debug("hook: sending queue message on session.end", "error", err)
			break
		}
		result.Actions = append(result.Actions, "queue.send")

	case "task.completed":
		// Move task to success column and notify.
		if event.TaskID != "" {
			if _, err := sdk.Dispatch(r.Context(), "task", []string{"finish", event.TaskID}, author, nil, ""); err != nil {
				slog.Debug("hook: finishing task", "task", event.TaskID, "error", err)
			} else {
				result.Actions = append(result.Actions, "task.finish")
			}
		}
		content := event.Content
		if content == "" && event.TaskID != "" {
			content = "Task " + event.TaskID + " completed"
		}
		if content != "" {
			args := []string{"send", content}
			if _, err := sdk.Dispatch(r.Context(), "queue", args, author, nil, ""); err != nil {
				slog.Debug("hook: sending completion message", "error", err)
			} else {
				result.Actions = append(result.Actions, "queue.send")
			}
		}

	case "task.failed":
		// Move task to failure column and alert.
		if event.TaskID != "" {
			if _, err := sdk.Dispatch(r.Context(), "task", []string{"move", event.TaskID, "blocked"}, author, nil, ""); err != nil {
				slog.Debug("hook: moving failed task to blocked", "task", event.TaskID, "error", err)
			} else {
				result.Actions = append(result.Actions, "task.move")
			}
		}
		content := event.Content
		if content == "" && event.TaskID != "" {
			content = "Task " + event.TaskID + " failed"
		}
		if content != "" {
			args := []string{"send", content}
			if _, err := sdk.Dispatch(r.Context(), "queue", args, author, nil, ""); err != nil {
				slog.Debug("hook: sending failure message", "error", err)
			} else {
				result.Actions = append(result.Actions, "queue.send")
			}
		}

	case "tool.post":
		// Log significant tool calls as audit entries.
		if event.TaskID != "" && event.Content != "" {
			args := []string{"add", event.TaskID}
			if _, err := sdk.Dispatch(r.Context(), "audit", args, author, []byte(event.Content), ""); err != nil {
				slog.Debug("hook: adding audit for tool call", "task", event.TaskID, "error", err)
			} else {
				result.Actions = append(result.Actions, "audit.add")
			}
		}

	default:
		slog.Debug("hook: unhandled event type", "event", event.Event, "author", author)
	}

	return result
}

// responseText extracts plain text from an sdk.Response for inclusion
// in hook response payloads. Markdown responses are emitted raw -
// hooks feed downstream tooling that wants the source, not terminal
// escape codes.
func responseText(r sdk.Response) string {
	switch v := r.(type) {
	case sdk.Text:
		return string(v)
	case sdk.Markdown:
		if v.Text != "" {
			return v.Text
		}
		b, err := json.Marshal(v.Data)
		if err != nil {
			return ""
		}
		return string(b)
	case sdk.Result:
		if v.Text != "" {
			return v.Text
		}
		b, err := json.Marshal(v.Data)
		if err != nil {
			return ""
		}
		return string(b)
	case sdk.Data:
		b, err := json.Marshal(v.V)
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return ""
	}
}
