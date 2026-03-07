# Getting Started with ingatan

## Prerequisites

- Go 1.22 or later
- An embedding API key (OpenAI recommended) for semantic search
- An LLM API key (Anthropic or OpenAI) for conversation summarization (optional)

## Build from Source

```bash
git clone https://github.com/stainedhead/ingatan.git
cd ingatan

# Build for your current platform
go build -o bin/ingatan ./cmd/ingatan

# Or cross-compile for Raspberry Pi
GOOS=linux GOARCH=arm64 go build -o bin/ingatan-arm64 ./cmd/ingatan
```

## Configuration

Create a minimal config file:

```bash
mkdir -p ~/.ingatan
cp config.example.yaml ~/.ingatan/config.yaml
```

Edit `~/.ingatan/config.yaml`. The minimum required settings:

```yaml
server:
  host: "0.0.0.0"
  port: 8443

data_dir: "~/.ingatan/data"

auth:
  algorithm: "HS256"
  # Set secret via environment variable (see below)
```

Set the JWT secret as an environment variable (never in the config file):

```bash
export INGATAN_AUTH_SECRET="your-secret-key-at-least-32-characters-long"
```

To enable semantic search, configure an embedding provider:

```yaml
embedding:
  provider: "openai"
  model: "text-embedding-3-small"
  dimensions: 1536
```

```bash
export INGATAN_EMBEDDING_API_KEY="sk-..."
```

## Running the Server

### Plain HTTP (development only)

```bash
./bin/ingatan --config ~/.ingatan/config.yaml
```

Output:

```
{"level":"INFO","msg":"embedding enabled","provider":"openai","model":"text-embedding-3-small"}
{"level":"INFO","msg":"starting ingatan server (plain HTTP — no TLS configured)","addr":"0.0.0.0:8443","version":"1.0.0-dev"}
```

### With TLS (recommended for production)

Add TLS certificate paths to your config:

```yaml
server:
  tls:
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

```bash
./bin/ingatan --config ~/.ingatan/config.yaml
```

## Create a JWT Token

ingatan expects a JWT bearer token on every request. Generate one using your configured secret. The token must include a `sub` claim (the principal ID).

Example using Python:

```python
import jwt, time
token = jwt.encode(
    {"sub": "alice", "name": "Alice", "type": "human", "role": "admin",
     "iat": int(time.time()), "exp": int(time.time()) + 86400},
    "your-secret-key-at-least-32-characters-long",
    algorithm="HS256"
)
print(token)
```

Or using the `jwt` CLI tool:

```bash
jwt encode --secret "your-secret-key-at-least-32-characters-long" \
  '{"sub":"alice","name":"Alice","type":"human","role":"admin"}'
```

Set the token for convenience:

```bash
export TOKEN="eyJhbG..."
```

## First Steps

### 1. Check server health

```bash
curl -s http://localhost:8443/health | jq
```

```json
{"status": "ok", "version": "1.0.0-dev"}
```

### 2. Check your identity

On first request, ingatan auto-creates your principal and a personal store:

```bash
curl -s http://localhost:8443/api/v1/principal/whoami \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 3. Create a store

```bash
curl -s -X POST http://localhost:8443/api/v1/stores \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-notes", "description": "Personal knowledge base"}' | jq
```

### 4. Save a memory

```bash
curl -s -X POST http://localhost:8443/api/v1/stores/my-notes/memories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Go error handling",
    "content": "In Go, errors are values. Use fmt.Errorf with %w to wrap errors for context. Always check returned errors.",
    "tags": ["go", "best-practices"],
    "source": "manual"
  }' | jq
```

### 5. Search memories

```bash
curl -s -X POST http://localhost:8443/api/v1/stores/my-notes/memories/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "error handling", "mode": "hybrid", "top_k": 5}' | jq
```

### 6. Save a URL

```bash
curl -s -X POST http://localhost:8443/api/v1/stores/my-notes/memories/url \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://go.dev/blog/error-handling-and-go", "tags": ["go", "errors"]}' | jq
```

## Connecting an MCP Client

ingatan serves MCP tools via streamable HTTP at `/mcp`. Configure your MCP client to connect:

**Claude Desktop** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ingatan": {
      "url": "http://localhost:8443/mcp",
      "headers": {
        "Authorization": "Bearer <your-jwt-token>"
      }
    }
  }
}
```

**Claude Code** (`.mcp.json`):

```json
{
  "mcpServers": {
    "ingatan": {
      "type": "streamable-http",
      "url": "http://localhost:8443/mcp",
      "headers": {
        "Authorization": "Bearer <your-jwt-token>"
      }
    }
  }
}
```

Once connected, the agent has access to all 23 MCP tools: memory management, search, ingest, store management, conversations, and more.

## Next Steps

- See [Configuration Reference](configuration.md) for all config options
- Enable TLS and mTLS for production deployments
- Configure backup providers (S3 or git) for data safety
- Set up an LLM provider for conversation summarization

## Admin WebUI

ingatan includes a browser-based admin console accessible from localhost only.

### Accessing the WebUI

1. Start ingatan. Look for this log line:
   ```
   INFO Admin WebUI enabled — localhost only url=http://localhost:8443/webui token=<64-char-hex>
   ```
2. Copy the token from the log.
3. Open `http://localhost:8443/webui` in your browser (must be on the same machine).
4. Paste the token into the login form and click **Sign in**.

The token is generated fresh on every restart. Sessions last 24 hours and are lost on restart.

### What You Can Do

| Section | Actions |
|---------|---------|
| **Dashboard** | Overview of principal and store counts |
| **Principals** | List, create, view details, reissue/revoke API keys |
| **Stores** | List stores, view members, delete non-personal stores |
| **System** | Trigger S3 or git backup manually |

### Disabling the WebUI

Set `webui.enabled: false` in your config file to disable the WebUI entirely.
