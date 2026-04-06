package generic

import "fmt"

// HookConfig returns a shell wrapper script that can be used with
// any agent that supports command hooks.
func HookConfig(_, author string) string {
	return fmt.Sprintf(`#!/bin/bash
# llmd hook wrapper for generic agents.
# Run your agent inside this script to get lifecycle updates.
#
# Usage: ./hooks.sh <task-id> <command...>

TASK_ID="$1"
shift

llmd --author %q task move "$TASK_ID" in-progress

"$@"
EXIT=$?

if [ $EXIT -eq 0 ]; then
  llmd --author %q queue send "Task $TASK_ID completed"
else
  llmd --author %q queue send "Task $TASK_ID failed (exit $EXIT)"
  llmd --author %q task move "$TASK_ID" blocked
fi

exit $EXIT
`, author, author, author, author)
}
