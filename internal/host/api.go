// api.go provides shared helpers used by the domain-specific API bridge
// files (api_documents.go, api_tasks.go, api_links.go, api_tags.go,
// api_activity.go).

package host

import "github.com/jpl-au/llmd/pkg/model/core"

// origin builds a core.Origin for CLI operations.
func origin(author string) core.Origin {
	return core.Origin{Author: author, Source: "cli"}
}
