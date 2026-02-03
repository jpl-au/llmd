# Observations: LLM Context & Memory Landscapes

This document compares **llmd** with existing solutions and analyzes its potential as a structured context layer for agents.

## Comparative Analysis

| Feature | [agents.md](https://agents.md/) | [memvid](https://github.com/memvid/memvid) | **llmd** (Proposed) |
| :--- | :--- | :--- | :--- |
| **Storage** | Flat Markdown File | Custom `.mv2` (Binary) | SQLite `.db` |
| **Retrieval** | Linear Scan (Full Read) | Vector/Temporal (Fuzzy) | SQL/FTS (Structured/Deterministic) |
| **Primary Use** | Static Instructions | Long-term "Fuzzy" Memory | **Dynamic Project Context** |
| **Structure** | Unstructured Text | Multi-modal Frames | Entities, Links, and Tags |

## Usefulness & Strategy

### 1. Solving "File System Litter"
Current agent workflows often rely on a directory full of `.md` files. This is fragile and noisy for version control. `llmd` treats documentation as **queryable data**, allowing for transactional integrity and a single portable artifact (`llmd.db`).

### 2. The Mixed-Use Case
`llmd` serves a dual purpose:
- **For Humans:** A CLI-first "Second Brain" for writing specs, plans, and documentation without cluttering the project root.
- **For Agents:** A high-fidelity context source. Unlike vector DBs (which provide "similar" data), `llmd` provides "exact" data through structured relationships (Links/Tags).

### 3. Critical Path: MCP Integration
The current architecture includes `internal/host/mcp.go`. Implementing the **Model Context Protocol (MCP)** is the "killer feature." It allows agents (like Claude Desktop or IDE extensions) to "mount" the documentation as a live toolset rather than reading raw files.

## References
- **memvid:** [https://github.com/memvid/memvid](https://github.com/memvid/memvid)
- **agents.md:** [https://agents.md/](https://agents.md/)

## Landscape Assessment (Market Scan)

A broader search reveals three distinct categories of tools. `llmd` occupies a specific niche in "Static Project Context" that is often confused with "Agent Runtime Memory."

### 1. Agent Runtime Memory (The "User Diary")
Tools like **MemGPT**, **mem0 (Memo)**, **SimpleMem**, and **LLM-Extended-Memory**.
*   **Goal:** Infinite conversation history and user personalization (e.g., "Remember I like Python").
*   **Mechanism:** Vector Databases (Chroma/Pinecone) + Graph. They manage *dynamic* state about the user or session.
*   **Comparison:** `llmd` is **not** this. `llmd` does not track chat history or user preferences.

### 2. Context Engineering & Packing
Tools like **llm-context**, **mq**, and **repopack**.
*   **Goal:** "Feed this codebase to the LLM."
*   **Mechanism:** File traversal + Concatenation.
*   **Comparison:** These are stateless and inefficient (re-reading files constantly). `llmd` is stateful (SQLite), allowing for faster, surgical queries.

### 3. Static Project Context (The "Project Manual")
This is where **llmd** fits, along with **agents.md** and **Search-based RAG**.
*   **Goal:** Answering "How does *this specific project* work?" (e.g., "Show me the API spec for `User`").
*   **Mechanism:** Structured Knowledge Graph.
*   **The `llmd` Advantage:**
    *   Unlike **Vectors** (which find "similar" things), `llmd` uses **SQL/FTS** to find *exact* definitions, which is critical for coding.
    *   Unlike **Markdown files** (which require parsing), `llmd` is a pre-indexed database.
    *   **MCP Integration:** By implementing the Model Context Protocol, `llmd` allows an agent to "mount" the project documentation as a tool, enabling autonomous exploration of the codebase's rules and specs.

### 4. The Platform Architecture (WASM)
Most competitors are monolithic scripts. `llmd` is an **extensible platform**.
*   **Mechanism:** It uses **WASM (WebAssembly)** for plugins (seen in `plugins/core/main.go`).
*   **Implication:** The "Core" commands (`cat`, `ls`, `grep`) are just a plugin. This means:
    *   **Safely Extensible:** Users can install 3rd-party plugins (e.g., a "Calendar" or "Vector Memory" plugin) without recompiling.
    *   **Language Agnostic:** Plugins can be written in Rust, Go, or TypeScript.
    *   **Universal Node:** `llmd` acts as a "Kernel" (handling SQLite/MCP), while plugins define the "Userland" behavior. This confirms it *can* be a "User Diary" if the right plugin is installed.

### Conclusion
`llmd` is a **Headless CMS for Agent Context**.
*   Use **MemGPT** if you want your agent to remember *you*.
*   Use **llmd** if you want your agent to understand *your project*.
