# WideMindsMCP

WideMinds MCP is an exploration-first knowledge navigation engine built around large language models and guided thought expansion. The backend is written in Go and delivers session management, thought-path generation, and MCP tooling. The frontend renders an interactive mind map so you can fan out from a single concept, pick a direction, and deepen your understanding step by step.

## Key Capabilities

- **Thought Expansion Engine** – Orchestrates an LLM to generate multi-dimensional directions and preview nodes for any concept.
- **Session Management** – Creates, retrieves, and persists user sessions while tracking node counts, depth, and direction coverage.
- **MCP Tooling** – Ships with `expand_thought`, `explore_direction`, `create_session`, and `get_session` tools, all callable over HTTP.
- **Visual Exploration** – Includes a thought tree and animated canvas with node highlighting, zoom, and pan interactions.

## Quick Start

### Prerequisites

- Go 1.22 or newer
- Node.js (optional, only if you plan to extend or rebuild the frontend assets)

### Clone the Repository

```powershell
# Windows PowerShell
cd <your-workspace>
git clone <repo-url>
cd WideMindsMCP
```

### Configure the Environment

1. Copy and edit the sample environment file:

   ```powershell
   copy configs\example.env .env
   # Update .env with values such as LLM_API_KEY when needed
   ```

2. Review `configs/config.yaml` to adjust ports, storage directories, or other defaults.

### Install Dependencies

```powershell
Set-Location WideMindsMCP
go mod tidy
```

### Run the Server

```powershell
Set-Location WideMindsMCP
# Launch the combined HTTP + MCP servers
go run ./cmd/server
```

After startup:
- Web UI is served at `http://localhost:8080`
- MCP endpoint listens on `http://localhost:9090`

### Explore the UI

Open `http://localhost:8080`, enter a seed concept (for example, “machine learning”), and generate expansion directions. Click **Deepen** on any direction to continue exploring the branch and watch the tree update in real time.

### API Endpoints

- `POST /api/sessions` – Create a session `{ "user_id": "u1", "concept": "Machine Learning" }`
- `GET /api/sessions/{id}` – Retrieve session details
- `POST /api/sessions/{id}` – Extend a session with a chosen direction `{ "direction": {...} }`
- `POST /api/expand` – Get expansion recommendations without mutating a session
- `POST /mcp` – Call an MCP tool with JSON payload `{"method": "expand_thought", "params": {...}}`
- `GET /tools` – List the registered MCP tools

## Quality & Testing

```powershell
Set-Location WideMindsMCP
# Optional: format source files
gofmt -w ./cmd ./internal
# Run unit tests
go test ./...
```

Unit tests cover the core models, session management flow, and storage layer to ensure thought paths and metadata stay consistent.

## Project Structure

- `cmd/server` – Application entry point; loads config, wires dependencies, starts HTTP/MCP services
- `internal/models` – Domain models (`Thought`, `Session`, `Direction`)
- `internal/services` – Business logic (`ThoughtExpander`, `LLMOrchestrator`, `SessionManager`)
- `internal/storage` – Session persistence (in-memory and file-backed implementations)
- `internal/mcp` – MCP server and tool wrappers
- `web/` – Frontend assets, including the thought tree and interactive canvas
- `configs/` – Configuration files and environment samples

## Roadmap

- Plug in a real LLM API via `LLMOrchestrator.CallLLM` for genuine model responses.
- Extend the front-end editor with drag connections, per-node annotations, and export formats.
- Introduce a production-grade database layer with stronger multi-user isolation.
- Experiment with advanced layout/physics engines (e.g., force-directed or layered graph layouts).

Contributions and feedback are welcome—open an issue or PR to help evolve the WideMinds navigation experience!
