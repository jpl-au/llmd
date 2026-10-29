// routes.go declares the HTTP routes for commands that take
// subcommands or more than one positional argument.
//
// The URL mirrors the command line: /<command>/<subcommand>/<args>.
// Reads are GET; anything that changes the store or the filesystem is
// POST, or DELETE for removals. A document path is always the last
// part of the URL, where it may contain slashes; when a route needs a
// document path earlier than that, it is URL-encoded as one segment
// (docs%2Fapi). Query parameters become flags and the request body is
// the command's input, as for every route.
//
// Commands not listed here use the server's default routes: GET
// /<command> and GET /<command>/{path...} for reads, POST for
// commands that need an author.

package cli

import "github.com/jpl-au/llmd/sdk"

func init() {
	// Document commands with more than a path. The second value comes
	// first, URL-encoded when it is a path, so the document path can
	// stay last; free text (old and new text, an expression) comes in
	// the query.
	mvSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/mv/{from}/{to...}", Args: []string{"{from}", "{to}"}},
	}
	revertSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/revert/{version}/{path...}", Args: []string{"{path}", "{version}"}},
	}
	editSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/edit/{path...}", Args: []string{"{path}", "{?old}", "{?new}"}},
	}
	sedSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/sed/{path...}", Args: []string{"{?expr}", "{path}"}},
	}

	tagSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/tag"},
		{Method: "GET", Pattern: "/tag/{path...}", Args: []string{"{path}"}},
		{Method: "POST", Pattern: "/tag/{name}/{path...}", Args: []string{"{path}", "{name}"}},
		{Method: "DELETE", Pattern: "/tag/{name}/{path...}", Args: []string{"--delete", "{path}", "{name}"}},
	}

	linkSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/link/{path...}", Args: []string{"{path}"}},
		{Method: "POST", Pattern: "/link/{from}/{to...}", Args: []string{"{from}", "{to}"}},
	}
	unlinkSpec.Routes = []sdk.Route{
		{Method: "DELETE", Pattern: "/link/{from}/{to...}", Args: []string{"{from}", "{to}"}},
	}

	taskSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/task", Args: []string{"list"}},
		{Method: "GET", Pattern: "/task/list", Args: []string{"list"}},
		{Method: "GET", Pattern: "/task/show/{id}", Args: []string{"show", "{id}"}},
		{Method: "GET", Pattern: "/task/links/{id}", Args: []string{"links", "{id}"}},
		{Method: "GET", Pattern: "/task/log/{id}", Args: []string{"log", "{id}"}},
		{Method: "GET", Pattern: "/task/chain/{id}", Args: []string{"chain", "{id}"}},
		{Method: "GET", Pattern: "/task/ready/{id}", Args: []string{"ready", "{id}"}},
		{Method: "GET", Pattern: "/task/diff/{id}", Args: []string{"diff", "{id}"}},
		{Method: "GET", Pattern: "/task/files/{id}", Args: []string{"files", "{id}"}},
		{Method: "GET", Pattern: "/task/commits/{id}", Args: []string{"commits", "{id}"}},
		{Method: "GET", Pattern: "/task/column", Args: []string{"column", "list"}},
		{Method: "POST", Pattern: "/task/add/{title...}", Args: []string{"add", "{title}"}},
		{Method: "POST", Pattern: "/task/move/{id}/{column}", Args: []string{"move", "{id}", "{column}"}},
		{Method: "POST", Pattern: "/task/next", Args: []string{"next"}},
		{Method: "POST", Pattern: "/task/set/{id}", Args: []string{"set", "{id}"}},
		{Method: "POST", Pattern: "/task/restore/{id}", Args: []string{"restore", "{id}"}},
		{Method: "POST", Pattern: "/task/link/{id}/{path...}", Args: []string{"link", "{id}", "{path}"}},
		{Method: "POST", Pattern: "/task/start/{id}", Args: []string{"start", "{id}"}},
		{Method: "POST", Pattern: "/task/finish/{id}", Args: []string{"finish", "{id}"}},
		{Method: "POST", Pattern: "/task/branch/{id}", Args: []string{"branch", "{id}"}},
		{Method: "POST", Pattern: "/task/column/add/{name}", Args: []string{"column", "add", "{name}"}},
		{Method: "POST", Pattern: "/task/column/mv/{name}", Args: []string{"column", "mv", "{name}"}},
		{Method: "DELETE", Pattern: "/task/column/{name}", Args: []string{"column", "rm", "{name}"}},
		{Method: "DELETE", Pattern: "/task/{id}", Args: []string{"rm", "{id}"}},
	}

	auditSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/audit", Args: []string{"list"}},
		{Method: "GET", Pattern: "/audit/list", Args: []string{"list"}},
		{Method: "GET", Pattern: "/audit/status", Args: []string{"status"}},
		{Method: "GET", Pattern: "/audit/show/{id}", Args: []string{"show", "{id}"}},
		{Method: "POST", Pattern: "/audit/add/{target...}", Args: []string{"add", "{target}"}},
		{Method: "POST", Pattern: "/audit/reply/{id}", Args: []string{"reply", "{id}"}},
		{Method: "POST", Pattern: "/audit/resolve/{id}", Args: []string{"resolve", "{id}"}},
		{Method: "POST", Pattern: "/audit/restore/{id}", Args: []string{"restore", "{id}"}},
		{Method: "DELETE", Pattern: "/audit/{id}", Args: []string{"rm", "{id}"}},
	}

	queueSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/queue", Args: []string{"ls"}},
		{Method: "GET", Pattern: "/queue/ls", Args: []string{"ls"}},
		{Method: "GET", Pattern: "/queue/peek", Args: []string{"peek"}},
		{Method: "GET", Pattern: "/queue/history", Args: []string{"history"}},
		{Method: "POST", Pattern: "/queue/send", Args: []string{"send"}},
		{Method: "POST", Pattern: "/queue/ack/{key}", Args: []string{"ack", "{key}"}},
	}

	agentSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/agent", Args: []string{"ls"}},
		{Method: "GET", Pattern: "/agent/ls", Args: []string{"ls"}},
		{Method: "GET", Pattern: "/agent/runs", Args: []string{"runs"}},
		{Method: "GET", Pattern: "/agent/config/{name}", Args: []string{"config", "{name}"}},
		{Method: "GET", Pattern: "/agent/prompt/{name}/{role}", Args: []string{"prompt", "{name}", "{role}"}},
		{Method: "POST", Pattern: "/agent/add/{name}", Args: []string{"add", "{name}"}},
		{Method: "POST", Pattern: "/agent/spawn/{task}/{name}", Args: []string{"spawn", "{task}", "{name}"}},
		{Method: "POST", Pattern: "/agent/complete/{task}", Args: []string{"complete", "{task}"}},
		{Method: "POST", Pattern: "/agent/stop/{task}", Args: []string{"stop", "{task}"}},
		{Method: "DELETE", Pattern: "/agent/{name}", Args: []string{"rm", "{name}"}},
	}

	mirrorSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/mirror/pull", Args: []string{"pull"}},
		{Method: "POST", Pattern: "/mirror/pull/{prefix...}", Args: []string{"pull", "{prefix}"}},
		{Method: "POST", Pattern: "/mirror/push", Args: []string{"push"}},
		{Method: "POST", Pattern: "/mirror/push/{prefix...}", Args: []string{"push", "{prefix}"}},
	}

	ruleSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/rule", Args: []string{"list"}},
		{Method: "POST", Pattern: "/rule/set/{column}", Args: []string{"set", "{column}"}},
		{Method: "DELETE", Pattern: "/rule/{column}", Args: []string{"unset", "{column}"}},
	}

	hookSpec.Routes = []sdk.Route{
		{Method: "GET", Pattern: "/hook/init/{platform}", Args: []string{"init", "{platform}"}},
		{Method: "POST", Pattern: "/hook"},
	}

	// export writes to the server's filesystem, so it is a POST even
	// though it does not change the store.
	exportSpec.Routes = []sdk.Route{
		{Method: "POST", Pattern: "/export/{path...}", Args: []string{"{path}"}},
	}
}
