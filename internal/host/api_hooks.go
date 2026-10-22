package host

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/validate"
	"github.com/jpl-au/llmd/sdk"
)

// hookAPI implements [sdk.HookStore]. It is the one place that decides
// what an agent lifecycle event does, built on the same queue, task
// and audit bridges every command uses.
type hookAPI struct {
	ctx   context.Context
	queue *queueAPI
	tasks *taskAPI
	audit *auditAPI
}

func newHookAPI(store *llmd.Store, lim validate.Limits, git sdk.GitStore, ctx context.Context) *hookAPI {
	return &hookAPI{
		ctx:   ctx,
		queue: newQueueAPI(store, ctx),
		tasks: newTaskAPI(store, lim, git, ctx),
		audit: newAuditAPI(store, ctx),
	}
}

func (h *hookAPI) Handle(author string, payload []byte, opts sdk.HookOpts) (*sdk.HookResult, error) {
	if author == "" {
		return nil, fmt.Errorf("%w: author", sdk.ErrMissingArg)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: hook payload", sdk.ErrMissingArg)
	}

	event, err := assets.Agent.Platform(author).ParseHook(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", sdk.ErrInvalidArg, err)
	}
	if event.TaskID == "" {
		event.TaskID = opts.Task
	}

	result := &sdk.HookResult{Event: event.Event, Task: event.TaskID}
	did := func(action string) { result.Actions = append(result.Actions, action) }
	send := func(content string) {
		if _, err := h.queue.Send(sdk.SendOpts{Type: "hook", Payload: content, Author: author, Source: "hook"}); err != nil {
			slog.Debug("hook: sending queue message", "event", event.Event, "error", err)
			return
		}
		did("queue.send")
	}
	about := func(what string) string {
		if event.Content != "" {
			return event.Content
		}
		if event.TaskID != "" {
			return what + " for task " + event.TaskID
		}
		return what
	}

	switch event.Event {
	case "session.start":
		msgs, err := h.queue.Pending(author, 0)
		if err != nil {
			slog.Debug("hook: listing pending messages", "error", err)
			break
		}
		did("queue.ls")
		lines := make([]string, 0, len(msgs))
		for _, m := range msgs {
			lines = append(lines, m.Key+"  "+m.Payload)
		}
		result.Content = strings.Join(lines, "\n")

	case "session.end", "session.stop":
		send(about("Session ended"))

	case "task.completed":
		if event.TaskID != "" {
			if _, err := h.tasks.Finish(event.TaskID, author, sdk.FinishOpts{}); err != nil {
				slog.Debug("hook: finishing task", "task", event.TaskID, "error", err)
			} else {
				did("task.finish")
			}
		}
		if event.Content != "" || event.TaskID != "" {
			send(about("Completed"))
		}

	case "task.failed":
		if event.TaskID != "" {
			if err := h.tasks.Move(event.TaskID, "blocked", author); err != nil {
				slog.Debug("hook: moving failed task to blocked", "task", event.TaskID, "error", err)
			} else {
				did("task.move")
			}
		}
		if event.Content != "" || event.TaskID != "" {
			send(about("Failed"))
		}

	case "tool.post":
		if event.TaskID != "" && event.Content != "" {
			if _, err := h.audit.Add(sdk.AuditOpts{Target: event.TaskID, Content: event.Content, Author: author}); err != nil {
				slog.Debug("hook: adding audit for tool call", "task", event.TaskID, "error", err)
			} else {
				did("audit.add")
			}
		}

	default:
		slog.Debug("hook: event not handled", "event", event.Event, "author", author)
	}

	return result, nil
}
