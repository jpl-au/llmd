// Package core provides the core plugin for llmd.
//
// The core plugin implements the standard document operations:
//
//   - cat: Read documents
//   - ls: List documents
//   - write: Create/update documents
//   - rm: Delete documents (soft delete)
//   - mv: Move/rename documents
//   - edit: Search/replace in documents
//   - grep: Full-text search
//   - glob: Find documents by path pattern
//   - history: Show version history
//   - diff: Compare document versions
//   - restore: Restore deleted documents
//   - revert: Revert to a previous version
//
// # Usage
//
// The core plugin is automatically registered by the host:
//
//	h := host.New(store)
//	result, _ := h.Exec("ls", []string{"-l"}, "", nil)
//
// # Flag Parsing
//
// Commands parse their own flags from the args slice. This allows Unix-style
// combined short flags (e.g., "ls -la") and keeps parsing logic with the
// command implementation.
//
// Example from ls:
//
//	for _, arg := range args {
//	    if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
//	        for _, c := range arg[1:] {
//	            switch c {
//	            case 'l': long = true
//	            case 'a': all = true
//	            }
//	        }
//	    }
//	}
//
// # Results
//
// Most commands return [sdk.Rich] with both text and structured data:
//
//   - Text output goes to the terminal
//   - Structured data is used for --json output
//
// Commands that only produce text (like write confirmations) return [sdk.Text].
package core
