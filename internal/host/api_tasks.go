package host

import (
	"context"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/pkg/model/task"
	"github.com/jpl-au/llmd/sdk"
)

// taskAPI implements sdk.TaskStore by delegating to the internal tasks package.
type taskAPI struct {
	store *llmd.Store
}

func newTaskAPI(store *llmd.Store) *taskAPI {
	return &taskAPI{store: store}
}

func taskToSDK(t *task.Task) *sdk.Task {
	return &sdk.Task{
		Key:        t.Key,
		Title:      t.Title,
		Status:     t.Status,
		Priority:   t.Priority,
		Position:   t.Position,
		AssignedTo: t.AssignedTo,
		Flags:      t.Flags,
		Path:       t.Path,
		Author:     t.Author,
		CreatedAt:  t.CreatedAt,
	}
}

func (a *taskAPI) Add(title string, body []byte, opts sdk.TaskAddOpts) (*sdk.Task, error) {
	t, err := a.store.Tasks.Add(context.Background(), title, body, tasks.AddOptions{
		Origin:     origin(opts.Author),
		Status:     opts.Status,
		Priority:   opts.Priority,
		AssignedTo: opts.AssignedTo,
		Path:       opts.Path,
	})
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *taskAPI) Read(key string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *taskAPI) List(opts sdk.TaskListOpts) ([]*sdk.Task, error) {
	tt, err := a.store.Tasks.List(context.Background(), tasks.ListOptions{
		Status:     opts.Status,
		AssignedTo: opts.AssignedTo,
		Priority:   opts.Priority,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*sdk.Task, len(tt))
	for i, t := range tt {
		out[i] = taskToSDK(t)
	}
	return out, nil
}

func (a *taskAPI) Move(key, column, author string) error {
	err := a.store.Tasks.Move(context.Background(), key, column, author)
	if errors.Is(err, tasks.ErrNoSpec) {
		return sdk.ErrNoSpec
	}
	return err
}

func (a *taskAPI) Set(key, author string, opts sdk.TaskSetOpts) error {
	return a.store.Tasks.Set(context.Background(), key, author, tasks.SetOptions{
		Title:      opts.Title,
		Priority:   opts.Priority,
		Position:   opts.Position,
		AssignedTo: opts.AssignedTo,
		Flag:       opts.Flag,
		Unflag:     opts.Unflag,
	})
}

func (a *taskAPI) Delete(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Delete(context.Background(), key, author)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *taskAPI) Restore(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Restore(context.Background(), key, author)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *taskAPI) Columns() ([]string, error) {
	return a.store.Tasks.Columns(context.Background())
}

func (a *taskAPI) AddColumn(name, after, author string) error {
	return a.store.Tasks.AddColumn(context.Background(), name, after, author)
}

func (a *taskAPI) RemoveColumn(name, author string) error {
	return a.store.Tasks.RemoveColumn(context.Background(), name, author)
}

func (a *taskAPI) MoveColumn(name, after, author string) error {
	return a.store.Tasks.MoveColumn(context.Background(), name, after, author)
}

func (a *taskAPI) Log(key string, limit int) ([]sdk.TaskEvent, error) {
	events, err := a.store.Tasks.Log(context.Background(), key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.TaskEvent, len(events))
	for i, e := range events {
		out[i] = sdk.TaskEvent{
			Timestamp: e.Timestamp,
			Subject:   e.Subject,
			Actor:     e.Actor,
			Action:    e.Action,
			OldValue:  e.OldValue,
			NewValue:  e.NewValue,
		}
	}
	return out, nil
}
