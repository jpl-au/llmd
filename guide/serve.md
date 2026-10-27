# llmd serve

Serve every llmd command over HTTP, with store events streamed to
clients.

## Usage

```
llmd serve
```

## Rules

- Listens on `localhost:5563`. Change it with `llmd config server.addr
  "localhost:9090"`.
- No authentication by default, which is fine on localhost. Before
  listening anywhere else, set `llmd config server.token
  "a-long-random-string"`; every request must then carry
  `Authorization: Bearer <token>` or it gets `401`.
- The URL mirrors the command line: `<METHOD> /<command>/<subcommand>/
  <arguments>`. Reads are `GET`; anything that changes the store or
  writes files is `POST`, or `DELETE` for removals. A `GET` never
  changes anything.
- The request body is the command's stdin. The `Author` header names
  who is acting; mutations without one are rejected unless the server
  has a default author. Query parameters become flags.
- A document path is always the last part of the URL. Encode it as one
  segment (`docs%2Fapi`) when a route needs it earlier.
- `mcp`, `serve`, `init`, `config`, `version` and `extensions` are not
  available over HTTP.
- Responses are plain text for text commands and JSON for structured
  ones; the `Output: json` header forces JSON. Errors are JSON with an
  `error` field. `GET /events` streams events (see `guide events`).
- Logs every request to stderr. Shuts down cleanly on SIGINT or
  SIGTERM.

## Examples

```bash
llmd config server.addr "localhost:9090"
llmd config server.token "a-long-random-string"
curl -H "Authorization: Bearer a-long-random-string" http://localhost:5563/ls
```

## How routes work

The URL mirrors the command line. The first part is the command, the
next is the subcommand when the command has one, and the rest are its
arguments:

```
<METHOD> /<command>/<subcommand>/<arguments>
```

Reads are `GET`. Anything that changes the store or writes files is
`POST`, or `DELETE` for removals. A `GET` request never changes
anything.

A document path is always the last part of the URL, so it can contain
slashes. When a route needs a document path earlier than that, encode
it as one segment: `docs%2Fapi`.

These commands are not available over HTTP: `mcp`, `serve`, `init`,
`config`, `version` and `extensions`.

## Route reference

### Documents

| Route                       | Description                         |
|-----------------------------|-------------------------------------|
| `GET /cat/<path>`           | Read a document                     |
| `GET /ls`                   | List all documents                  |
| `GET /ls/<prefix>`          | List documents under a prefix       |
| `GET /grep?q=<query>`       | Full-text search                    |
| `GET /find?q=<query>`       | Full-text search, paths only        |
| `GET /glob/<pattern>`       | Match documents by path pattern     |
| `GET /history/<path>`       | Version history for a document      |
| `GET /diff/<path>`          | Diff against the previous version   |
| `GET /graph`                | Version timeline for the store      |
| `GET /graph/<path>`         | Version history as a tree           |
| `GET /status`               | Store overview                      |
| `GET /review`               | Pending tasks with context          |
| `POST /write/<path>`        | Create or update a document         |
| `POST /edit/<path>`         | Search and replace in a document    |
| `POST /sed/<path>`          | Sed-style substitution              |
| `POST /rm/<path>`           | Soft-delete a document              |
| `POST /mv/<from>`           | Move or rename a document           |
| `POST /restore/<path>`      | Recover a deleted document          |
| `POST /revert/<path>`       | Roll back to a previous version     |
| `POST /import`              | Import .md files from a directory   |
| `POST /export/<path>`       | Export documents to the filesystem  |
| `POST /vacuum`              | Purge deleted documents             |

### Tags and links

| Route                          | Description                       |
|--------------------------------|-----------------------------------|
| `GET /tag`                     | List every tag with its count     |
| `GET /tag/<path>`              | List tags on a document           |
| `GET /tag/<name>?find=true`    | List documents carrying a tag     |
| `POST /tag/<name>/<path>`      | Add a tag to a document           |
| `DELETE /tag/<name>/<path>`    | Remove a tag from a document      |
| `GET /link/<path>`             | List links from a document        |
| `GET /link/<path>?in=true`     | List links to a document          |
| `GET /link/<path>?both=true`   | List links in both directions     |
| `POST /link/<from>/<to>`       | Create a link (`from` is encoded) |
| `DELETE /link/<from>/<to>`     | Remove a link (`from` is encoded) |

### Tasks

| Route                               | Description                        |
|-------------------------------------|------------------------------------|
| `GET /task`                         | Board view of every task           |
| `GET /task/list`                    | List tasks (filters as queries)    |
| `GET /task/show/<id>`               | Task details and spec              |
| `GET /task/links/<id>`              | Documents linked to a task         |
| `GET /task/log/<id>`                | Change history of a task           |
| `GET /task/chain/<id>`              | Dependency chain                   |
| `GET /task/ready/<id>`              | Whether dependencies are done      |
| `GET /task/diff/<id>`               | Git diff for the task's branch     |
| `GET /task/files/<id>`              | Files changed on the branch        |
| `GET /task/commits/<id>`            | Commits on the branch              |
| `GET /task/column`                  | List board columns                 |
| `POST /task/add/<title>`            | Create a task (body is the spec)   |
| `POST /task/move/<id>/<column>`     | Move a task to a column            |
| `POST /task/next`                   | Claim the first ready task (`?from=`, `?to=`, `?assign=`) |
| `POST /task/set/<id>`               | Update task fields (as queries)    |
| `POST /task/restore/<id>`           | Restore a deleted task             |
| `POST /task/link/<id>/<path>`       | Link a task to a document          |
| `POST /task/start/<id>`             | Start a task                       |
| `POST /task/finish/<id>`            | Finish a task                      |
| `POST /task/branch/<id>`            | Create a branch and start the task |
| `POST /task/column/add/<name>`      | Add a column (`?after=<column>`)   |
| `POST /task/column/mv/<name>`       | Reorder a column (`?after=`)       |
| `DELETE /task/column/<name>`        | Remove an empty column             |
| `DELETE /task/<id>`                 | Soft-delete a task                 |

### Audits

| Route                          | Description                          |
|--------------------------------|--------------------------------------|
| `GET /audit`                   | List audits                          |
| `GET /audit/list`              | List audits (filters as queries)     |
| `GET /audit/status`            | Inbox for the `Author` header        |
| `GET /audit/show/<id>`         | Show an audit thread                 |
| `POST /audit/add/<target>`     | Create an audit (body is the text)   |
| `POST /audit/reply/<id>`       | Reply to a thread (body is the text) |
| `POST /audit/resolve/<id>`     | Mark a thread approved               |
| `POST /audit/restore/<id>`     | Recover a deleted audit              |
| `DELETE /audit/<id>`           | Soft-delete an audit                 |

### Queue

| Route                      | Description                            |
|----------------------------|----------------------------------------|
| `GET /queue`               | Pending messages for the `Author`      |
| `GET /queue/ls`            | The same, with `?limit=<n>`            |
| `GET /queue/peek`          | Oldest pending message                 |
| `GET /queue/history`       | All messages, including acknowledged   |
| `POST /queue/send`         | Send a message (body is the content)   |
| `POST /queue/ack/<key>`    | Acknowledge the oldest pending message |

### Agents

| Route                               | Description                     |
|-------------------------------------|---------------------------------|
| `GET /agent`                        | List registered agents          |
| `GET /agent/ls`                     | The same                        |
| `GET /agent/runs`                   | List agent runs                 |
| `GET /agent/config/<name>`          | Show an agent's configuration   |
| `GET /agent/prompt/<name>/<role>`   | Show a prompt template          |
| `POST /agent/add/<name>`            | Register an agent               |
| `POST /agent/spawn/<task>/<name>`   | Spawn an agent for a task       |
| `POST /agent/complete/<task>`       | Record an agent's exit          |
| `POST /agent/stop/<task>`           | Stop a running agent            |
| `DELETE /agent/<name>`              | Remove an agent                 |

### Rules and mirror

| Route                          | Description                             |
|--------------------------------|-----------------------------------------|
| `GET /rule`                    | Show column rules                       |
| `POST /rule/set/<column>`      | Set a column rule (fields as queries)   |
| `DELETE /rule/<column>`        | Remove the agent from a column rule     |
| `POST /mirror/pull`            | Write every document to the mirror      |
| `POST /mirror/pull/<prefix>`   | Write documents under a prefix          |
| `POST /mirror/push`            | Import edits made in the mirror         |
| `POST /mirror/push/<prefix>`   | Import edits under a prefix             |

### Hooks

| Route                          | Description                              |
|--------------------------------|------------------------------------------|
| `POST /hook`                   | Handle an agent hook event (body is the payload) |
| `GET /hook/init/<platform>`    | Print a hook configuration               |

`POST /hook` is the `hook` command over HTTP. The body is the agent
platform's payload and the `Author` header names the agent, which
selects the payload format. The response is the text a command hook
would print; with `Output: json` it is the event, the task and the
actions taken. Pass `?task=<id>` when the payload does not name the
task. See `llmd guide hooks` for what each event does.

```bash
curl -X POST http://localhost:5563/hook \
  -H "Author: claude-code" \
  -d '{"hook_event_name": "SessionEnd", "session_id": "abc123"}'
```

## Headers

| Header   | Description                                              |
|----------|----------------------------------------------------------|
| `Author` | Identity for mutations. Falls back to the server default |
| `Authorization` | `Bearer <token>` when `server.token` is set    |
| `Message`| Version message (passed as `--message`)                  |
| `Source` | Source attribution (passed as `--source`)                 |
| `Output` | Set to `json` to force JSON response on all endpoints    |

## Query parameters

Query parameters are mapped to command flags automatically:

```
GET /cat/docs/spec?version=3     -> cat --version 3 docs/spec
GET /ls?l=true&t=true            -> ls -l -t
GET /grep?q=budget&n=true        -> grep -n budget
GET /history/docs/spec?limit=5   -> history --limit 5 docs/spec
```

The `q` parameter is special: it becomes a positional argument (the
search pattern) rather than a flag. Every other parameter becomes a
`--key value` flag. For a boolean flag, pass `true` to set it; a value
of `false` is ignored.

## Request body

The request body is passed as stdin to the command. This is how document
content is sent for `write`, `edit`, and similar commands:

```bash
curl -X POST http://localhost:5563/write/docs/readme \
  -H "Author: Claude" \
  -H "Message: initial draft" \
  -d "# Project README

This is the project documentation."
```

## Response format

By default, text commands return `text/plain` and structured commands
return `application/json`. Set the `Output: json` header to force JSON
on all endpoints.

Errors return JSON with an `error` field:

```json
{"error": "not found: docs/nonexistent"}
```

### HTTP status codes

| Code | Meaning                                          |
|------|--------------------------------------------------|
| 200  | Success                                          |
| 204  | Success, no content                              |
| 400  | Missing or invalid argument                      |
| 404  | Document or resource not found                   |
| 409  | Conflict (e.g. document already exists for mv)   |
| 422  | Unprocessable (e.g. task missing spec)            |
| 500  | Internal error                                   |

## Examples

### Read a document

```bash
curl http://localhost:5563/cat/docs/readme
```

### Read a specific version

```bash
curl "http://localhost:5563/cat/docs/readme?version=2"
```

### Write a document

```bash
curl -X POST http://localhost:5563/write/docs/readme \
  -H "Author: Claude" \
  -d "# Updated content"
```

### Search

```bash
curl "http://localhost:5563/grep?q=authentication"
```

### List documents as JSON

```bash
curl -H "Output: json" http://localhost:5563/ls
```

### List documents under a prefix

```bash
curl http://localhost:5563/ls/projects/
```

### Version history

```bash
curl http://localhost:5563/history/docs/readme
```

### Delete a document

```bash
curl -X POST http://localhost:5563/rm/docs/old-draft \
  -H "Author: Claude"
```

### Add and remove a tag

```bash
curl -X POST http://localhost:5563/tag/important/docs/readme \
  -H "Author: Claude"

curl -X DELETE http://localhost:5563/tag/important/docs/readme \
  -H "Author: Claude"
```

### Move a task

```bash
curl -X POST http://localhost:5563/task/move/0mvj8snvv/review \
  -H "Author: Claude"
```

### Link two documents

The first path is URL-encoded because it is not the last part of the URL.

```bash
curl -X POST http://localhost:5563/link/docs%2Freadme/docs/design \
  -H "Author: Claude"
```

### Create an audit

```bash
curl -X POST http://localhost:5563/audit/add/docs/spec \
  -H "Author: Gemini" \
  -d "Error handling in section 3 is incomplete."
```

## Notes

- The server is a thin transport layer over the host dispatcher, the same
  dispatch mechanism used by the CLI and MCP server. Behaviour is
  identical across all three interfaces.
- Plugin commands that register via the extension system get HTTP
  routes automatically.
- The server uses the `chain` router, which wraps the standard
  `net/http` mux. Routes are declared on each command (see
  `cli/routes.go`); commands without a declaration get `GET` or
  `POST /<command>/<path>` depending on whether they need an author.
