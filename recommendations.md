# Recommendations

Based on codebase analysis, the following areas are identified for immediate improvement.

## 1. Search & Retrieval Optimization

**Problem:** The current `grep` and `search` implementations are inefficient.
*   **Heavy Payload:** They query and return the **full document content** from SQLite for every match.
*   **Poor Context:**
    *   `grep` hardcodes `Line: 1` and returns the whole file.
    *   `search` returns the first 200 characters of the file, regardless of where the match actually occurred.

**Solution:** Offload processing to SQLite's FTS5 engine.
*   **Use `snippet()`:** Update the SQL query to use `snippet(content_fts, ...)` to retrieve only the relevant text window around the match.
*   **Use `offsets()`:** Use `offsets()` to calculate accurate line numbers and highlighting positions.
*   **Benefit:** dramatically reduces data transfer (SQLite -> Host -> WASM) and provides useful "Google-style" search results.

## 2. Platform Architecture (MCP Integration)

**Problem:** The `internal/host/mcp.go` file contains only TODOs.
*   **Impact:** The "Headless CMS" capability is currently theoretical. External agents cannot natively query the system.

**Solution:** Implement the MCP Server.
*   **Step 1:** Import `github.com/mark3labs/mcp-go`.
*   **Step 2:** Expose `Search`, `Read`, and `List` as MCP Tools.
*   **Benefit:** Allows tools like Claude Desktop or IDEs to "mount" the `llmd` database as a live context source.

## 3. Flexible Search API & Data Pipeline

**Problem:** The current Search API is "all or nothing."
*   `grep` fetches the entire document to find a single line.
*   `search` returns a fixed 200-char snippet, which might miss the relevant context.
*   There is no way to ask for "just metadata" (fast) vs "full content" (slow).

**Solution:** Refactor the Host <-> Plugin Interface for granular retrieval.

1.  **Structured Query API:**
    *   Update `SearchRequest` in `host.proto` to support detailed filtering and formatting options.
    *   Example:
        ```protobuf
        message SearchRequest {
          string query = 1;
          Filter filter = 2; // e.g. { tags: ["bug"], author: "jpl" }
          ResultFormat format = 3; // e.g. SNIPPET, FULL, METADATA_ONLY
          SnippetOptions snippet_config = 4; // e.g. { length: 500, highlight_tags: "<b>" }
        }
        ```

2.  **The "Kitchen Sink vs. Surgical" Pipeline:**
    *   **Host Layer (SQL):** Should execute the complex logic (FTS offsets, snippets, filtering) *inside* the database to minimize data transfer.
    *   **Plugin Layer (WASM):** Should receive *exactly* what it asked for.
        *   *Scenario A (User CLI):* Plugin asks for `ResultFormat: SNIPPET`. Host returns lightweight results. Plugin pretty-prints them for the terminal.
        *   *Scenario B (Agent/MCP):* Plugin asks for `ResultFormat: FULL`. Host returns complete docs. Plugin passes them to the LLM context window.

3.  **Benefit:**
    *   **Performance:** Massive speedup for large repos (fetching 10kb of snippets vs 100MB of raw text).
    *   **Usability:** Agents can first "scan" (metadata/headers) and then "read" (full content) only what they need, saving tokens.

## 4. API Maturity & Data Model

**Problem:** The current API is "chatty" and "file-centric" rather than "knowledge-centric."
*   **Fragmented Data:** To get a full picture of a document, a client currently needs to call:
    1.  `DocumentRead` (gets content)
    2.  `TagList` (gets tags)
    3.  `LinkList` (gets connections)
*   **Rigid Listing:** `DocumentList` only supports filtering by `prefix`. It is impossible to ask: "Give me all documents tagged 'bug' written by 'jpl' in the last week."
*   **Opaque Metadata:** `Document` returns raw bytes. It does not natively handle frontmatter or structured metadata, forcing every plugin to re-implement parsing.

**Solution:** "Thick" Responses and Filter-Based Listing.

1.  **Unified Document Response:**
    *   Update `ReadRequest` to accept `FetchOptions` (e.g., `include_tags`, `include_links`, `parse_frontmatter`).
    *   Update `Document` message to include these fields if requested.
    *   **Benefit:** Reduces RTT (Round Trip Time) between Plugin and Host.

2.  **Advanced Query Language (AQL):**
    *   Replace `prefix` in `ListRequest` with a structured `Filter` message.
        ```protobuf
        message Filter {
          repeated string tags_contains = 1;
          string author_matches = 2;
          TimeRange created_between = 3;
          map<string, string> metadata_equals = 4; // e.g. status=draft
        }
        ```
    *   **Benefit:** Turns `llmd` into a database, not just a file system.

## 5. The "Raw SQL" Access Question

**Question:** *Can/Should plugins access the underlying SQLite connection directly?*

**Verdict:** **Strictly No** (for the Core Standard).

**Reasoning:**
1.  **Security:** Exposing raw SQL allows a malicious (or buggy) plugin to bypass all ACLs, delete the entire database, or perform SQL injection attacks.
2.  **Abstraction Leak:** If a plugin writes `SELECT * FROM content`, it effectively locks `llmd` to SQLite forever. If you later migrate the backend to PostgreSQL or a distributed store, that plugin breaks.
3.  **Schema Coupling:** Plugins would need to know the internal table structure (`content`, `content_fts`), which prevents you from refactoring the core database schema in the future without breaking the ecosystem.

**The "Pro" Exception (Extensibility Strategy):**
*   If an "Escape Hatch" is absolutely required for advanced plugins (e.g., a "Database Admin" plugin), do **not** expose it by default.
*   **Implementation:** Use a **Capability System** in the Plugin Manifest.
    *   `manifest.json`: `"capabilities": ["host:sql_unsafe"]`
    *   The Host must explicitly grant this permission at load time (prompting the user).
    *   This creates a clear boundary: "Standard plugins are safe; Unsafe plugins require manual approval."

## 6. Batch Operations

**Problem:** `Write` and `Read` are single-item operations.
*   **Impact:** Bulk importing documentation or indexing large repos is slow due to overhead per call.

**Recommendation:** Add `BatchRead` and `BatchWrite` RPCs.
*   Allow plugins to send a stream or array of documents.
*   Host handles them in a single SQLite transaction for massive performance gains.
