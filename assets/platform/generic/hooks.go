package generic

import (
	"fmt"

	"github.com/jpl-au/llmd/assets/platform"
)

// HookConfig returns a shell script for agents without a hook system.
// It runs the agent command and reports the task's lifecycle to llmd
// by sending generic hook payloads to "llmd hook": session.start
// before the command, then task.completed or task.failed.
func HookConfig(author string) string {
	return fmt.Sprintf(`#!/bin/sh
# llmd hook wrapper for agents without a hook system.
#
# Usage: ./hooks.sh <task-id> <command...>
#
# Reports the task's lifecycle to llmd through "llmd hook": a session
# start before the command runs, then task completed or task failed.

TASK_ID="$1"
shift

emit() {
  printf '{"event":"%%s","task_id":"%%s","content":"%%s"}' "$1" "$TASK_ID" "$2" | %s --task "$TASK_ID"
}

emit session.start ""

"$@"
EXIT=$?

if [ $EXIT -eq 0 ]; then
  emit task.completed "Task $TASK_ID completed"
else
  emit task.failed "Task $TASK_ID failed (exit $EXIT)"
fi

exit $EXIT
`, platform.HookCommand(author))
}
