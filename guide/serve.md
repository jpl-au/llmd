# llmd serve

Start an HTTP API server exposing llmd commands as REST endpoints.

## Usage

```
llmd serve
```

The server listens on `localhost:8080` by default. To change the address,
set `serve_addr` in your store config:

```
llmd config serve_addr "localhost:9090"
```

## How routes work

Every registered command becomes an HTTP route automatically. Read
commands are `GET`, mutation commands are `POST`. The URL pattern is:

```
<METHOD> /<command>/<path>
```

Commands that don't make sense over HTTP are excluded: `mcp`, `serve`,
`init`, `config`, `version`, `plugins`, `guide`, and `llm`.

## Route reference

### Read commands (GET)

| Route                        | Description                         |
|------------------------------|-------------------------------------|
| `GET /cat/<path>`            | Read a document                     |
| `GET /ls`                    | List all documents                  |
| `GET /ls/<prefix>`           | List documents under a prefix       |
| `GET /grep?q=<query>`       | Full-text search                    |
| `GET /find?q=<query>`       | Full-text search, paths only        |
| `GET /glob/<pattern>`       | Match documents by path pattern     |
| `GET /history/<path>`       | Version history for a document      |
| `GET /diff/<path>`          | Diff against previous version       |
| `GET /tag/<path>`           | List tags on a document             |
| `GET /link/<path>`          | List links on a document            |
| `GET /status`               | Store overview dashboard            |
| `GET /review`               | Pending tasks with context          |
| `GET /task/list`            | List tasks                          |
| `GET /audit/list`           | List audits                         |
| `GET /audit/show/<id>`     | Display full audit thread           |
| `GET /audit/status`        | Pending audits inbox                |

### Mutation commands (POST)

| Route                        | Description                         |
|------------------------------|-------------------------------------|
| `POST /write/<path>`        | Create or update a document         |
| `POST /edit/<path>`         | Search and replace in a document    |
| `POST /sed/<path>`          | Sed-style substitution              |
| `POST /rm/<path>`           | Soft-delete a document              |
| `POST /mv/<from>`           | Move or rename a document           |
| `POST /restore/<path>`      | Recover a deleted document          |
| `POST /revert/<path>`       | Roll back to a previous version     |
| `POST /import`              | Import .md files from a directory   |
| `POST /link/<from>`         | Create a link                       |
| `POST /unlink/<from>`       | Remove a link                       |
| `POST /tag/<path>`          | Add a tag                           |
| `POST /task/add`            | Create a task                       |
| `POST /audit/add`           | Create an audit                     |
| `POST /audit/reply/<id>`   | Reply to an audit thread            |
| `POST /audit/resolve/<id>` | Mark audit as approved              |
| `POST /audit/rm/<id>`      | Soft-delete an audit                |
| `POST /audit/restore/<id>` | Recover a deleted audit             |

## Headers

| Header   | Description                                              |
|----------|----------------------------------------------------------|
| `Author` | Identity for mutations. Falls back to the server default |
| `Message`| Version message (passed as `--message`)                  |
| `Source` | Source attribution (passed as `--source`)                 |
| `Output` | Set to `json` to force JSON response on all endpoints    |

## Query parameters

Query parameters are mapped to command flags automatically:

```
GET /cat/docs/spec?version=3     -> cat --version 3 docs/spec
GET /ls?l=true&t=true            -> ls -l -t
GET /grep?q=budget&n=true        -> grep -n budget
GET /history/docs/spec?n=5       -> history -n 5 docs/spec
```

The `q` parameter is special — it becomes a positional argument (the
search pattern) rather than a flag. All other parameters become `--key
value` flags.

## Request body

The request body is passed as stdin to the command. This is how document
content is sent for `write`, `edit`, and similar commands:

```bash
curl -X POST http://localhost:8080/write/docs/readme \
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
curl http://localhost:8080/cat/docs/readme
```

### Read a specific version

```bash
curl "http://localhost:8080/cat/docs/readme?version=2"
```

### Write a document

```bash
curl -X POST http://localhost:8080/write/docs/readme \
  -H "Author: Claude" \
  -d "# Updated content"
```

### Search

```bash
curl "http://localhost:8080/grep?q=authentication"
```

### List documents as JSON

```bash
curl -H "Output: json" http://localhost:8080/ls
```

### List documents under a prefix

```bash
curl http://localhost:8080/ls/projects/
```

### Version history

```bash
curl http://localhost:8080/history/docs/readme
```

### Delete a document

```bash
curl -X POST http://localhost:8080/rm/docs/old-draft \
  -H "Author: Claude"
```

### Add a tag

```bash
curl -X POST "http://localhost:8080/tag/docs/readme?q=important" \
  -H "Author: Claude"
```

### Create an audit

```bash
curl -X POST http://localhost:8080/audit/add/docs/spec \
  -H "Author: Gemini" \
  -d "Error handling in section 3 is incomplete."
```

## Notes

- The server is a thin transport layer over `sdk.Dispatch`, the same
  dispatch mechanism used by the CLI and MCP server. Behaviour is
  identical across all three interfaces.
- Plugin commands that register via the extension system get HTTP
  routes automatically.
- The server uses the `chain` router, which wraps Go 1.22's stdlib
  `net/http` mux with method-based routing and `{path...}` wildcards.
