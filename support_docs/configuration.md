# Configuration Reference

ingatan uses a YAML configuration file with environment variable overrides.

## Config File Location

Default: `~/.ingatan/config.yaml`

Override with the `--config` flag:

```bash
./bin/ingatan --config /path/to/config.yaml
```

## Environment Variable Override

All config fields can be overridden with `INGATAN_` prefixed environment variables. Nested keys use underscores:

```
server.port          -> INGATAN_SERVER_PORT
auth.secret          -> INGATAN_AUTH_SECRET
embedding.api_key    -> INGATAN_EMBEDDING_API_KEY
llm.api_key          -> INGATAN_LLM_API_KEY
server.tls.cert_file -> INGATAN_SERVER_TLS_CERT_FILE
```

Environment variables take precedence over the YAML file.

## Secrets Handling

Never put secrets in the config file. Use environment variables for:

- `INGATAN_AUTH_SECRET` -- JWT signing secret (HS256)
- `INGATAN_EMBEDDING_API_KEY` -- Embedding provider API key
- `INGATAN_LLM_API_KEY` -- LLM provider API key

## Complete Configuration Reference

### server

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `server.host` | string | `"0.0.0.0"` | `INGATAN_SERVER_HOST` | Bind address |
| `server.port` | int | `8443` | `INGATAN_SERVER_PORT` | Listen port |

### server.tls

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `server.tls.cert_file` | string | `""` | `INGATAN_SERVER_TLS_CERT_FILE` | TLS certificate path. Empty = plain HTTP. |
| `server.tls.key_file` | string | `""` | `INGATAN_SERVER_TLS_KEY_FILE` | TLS private key path |
| `server.tls.client_ca` | string | `""` | `INGATAN_SERVER_TLS_CLIENT_CA` | Client CA PEM for mTLS. Empty = no mTLS. |
| `server.tls.min_version` | string | `"1.2"` | `INGATAN_SERVER_TLS_MIN_VERSION` | Minimum TLS version: `"1.2"` or `"1.3"` |

### data_dir

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `data_dir` | string | `"~/.ingatan/data"` | `INGATAN_DATA_DIR` | Root directory for all persisted data |

### auth

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `auth.algorithm` | string | `"HS256"` | `INGATAN_AUTH_ALGORITHM` | JWT algorithm: `"HS256"` or `"RS256"` |
| `auth.secret` | string | `""` | `INGATAN_AUTH_SECRET` | HS256 signing secret. **Use env var only.** |
| `auth.public_key` | string | `""` | `INGATAN_AUTH_PUBLIC_KEY` | RS256 public key PEM path |
| `auth.issuer` | string | `""` | `INGATAN_AUTH_ISSUER` | Optional JWT issuer claim to validate |

### embedding

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `embedding.provider` | string | `""` | `INGATAN_EMBEDDING_PROVIDER` | Provider: `"openai"`, `"bedrock"`, `"ollama"` |
| `embedding.model` | string | `""` | `INGATAN_EMBEDDING_MODEL` | Model name (e.g., `"text-embedding-3-small"`) |
| `embedding.dimensions` | int | `1536` | `INGATAN_EMBEDDING_DIMENSIONS` | Vector dimensions |
| `embedding.api_key` | string | `""` | `INGATAN_EMBEDDING_API_KEY` | API key. **Use env var only.** |
| `embedding.base_url` | string | `""` | `INGATAN_EMBEDDING_BASE_URL` | Custom endpoint (Ollama, compatible APIs) |

### llm

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `llm.provider` | string | `""` | `INGATAN_LLM_PROVIDER` | Provider: `"anthropic"`, `"openai"`, `"ollama"` |
| `llm.model` | string | `""` | `INGATAN_LLM_MODEL` | Model name (e.g., `"claude-3-5-haiku-latest"`) |
| `llm.api_key` | string | `""` | `INGATAN_LLM_API_KEY` | API key. **Use env var only.** |
| `llm.base_url` | string | `""` | `INGATAN_LLM_BASE_URL` | Custom endpoint (Ollama base URL) |

### chunking

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `chunking.strategy` | string | `"recursive"` | `INGATAN_CHUNKING_STRATEGY` | Chunking strategy (only `"recursive"` supported) |
| `chunking.chunk_size` | int | `512` | `INGATAN_CHUNKING_CHUNK_SIZE` | Tokens per chunk (character-based approximation) |
| `chunking.chunk_overlap` | int | `64` | `INGATAN_CHUNKING_CHUNK_OVERLAP` | Overlap tokens between chunks |
| `chunking.max_content_bytes` | int | `10485760` | `INGATAN_CHUNKING_MAX_CONTENT_BYTES` | Max content size (10 MiB default) |

### ingest

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `ingest.allowed_paths` | []string | `[]` | -- | Allowed directories for file ingest. Empty = deny all. |
| `ingest.max_content_bytes` | int | `10485760` | `INGATAN_INGEST_MAX_CONTENT_BYTES` | Max content size for URL/file ingest |
| `ingest.robots_txt_compliance` | bool | `true` | `INGATAN_INGEST_ROBOTS_TXT_COMPLIANCE` | Respect robots.txt for URL fetching |

### conversation

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `conversation.auto_summarize_message_threshold` | int | `50` | `INGATAN_CONVERSATION_AUTO_SUMMARIZE_MESSAGE_THRESHOLD` | Auto-summarize after N messages. 0 = disabled. |
| `conversation.auto_summarize_token_estimate_threshold` | int | `8000` | `INGATAN_CONVERSATION_AUTO_SUMMARIZE_TOKEN_ESTIMATE_THRESHOLD` | Auto-summarize after estimated N tokens. 0 = disabled. |

### rate_limit

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `rate_limit.requests_per_minute` | int | `300` | `INGATAN_RATE_LIMIT_REQUESTS_PER_MINUTE` | Requests per minute per IP |
| `rate_limit.burst_size` | int | `50` | `INGATAN_RATE_LIMIT_BURST_SIZE` | Burst allowance per IP |

### backup.s3

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `backup.s3.enabled` | bool | `false` | `INGATAN_BACKUP_S3_ENABLED` | Enable S3 backup |
| `backup.s3.bucket` | string | `""` | `INGATAN_BACKUP_S3_BUCKET` | S3 bucket name |
| `backup.s3.region` | string | `""` | `INGATAN_BACKUP_S3_REGION` | AWS region |
| `backup.s3.prefix` | string | `"ingatan-backup/"` | `INGATAN_BACKUP_S3_PREFIX` | Key prefix in bucket |
| `backup.s3.schedule` | string | `"0 2 * * *"` | `INGATAN_BACKUP_S3_SCHEDULE` | Cron schedule (informational) |

### backup.git

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `backup.git.enabled` | bool | `false` | `INGATAN_BACKUP_GIT_ENABLED` | Enable git backup |
| `backup.git.remote_url` | string | `""` | `INGATAN_BACKUP_GIT_REMOTE_URL` | Git remote URL for push |
| `backup.git.branch` | string | `"backup"` | `INGATAN_BACKUP_GIT_BRANCH` | Branch name |
| `backup.git.schedule` | string | `"0 3 * * *"` | `INGATAN_BACKUP_GIT_SCHEDULE` | Cron schedule (informational) |

### telemetry

| Field | Type | Default | Env Var | Description |
|-------|------|---------|---------|-------------|
| `telemetry.otel_endpoint` | string | `""` | `INGATAN_TELEMETRY_OTEL_ENDPOINT` | OTLP gRPC endpoint. `"stdout"` for dev. Empty = disabled. |
| `telemetry.service_name` | string | `"ingatan"` | `INGATAN_TELEMETRY_SERVICE_NAME` | OTel service name |

## TLS Setup

### Basic TLS

Generate a self-signed certificate for development:

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/CN=ingatan"
```

Configure:

```yaml
server:
  tls:
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

### mTLS (Mutual TLS)

For environments where clients must present certificates:

1. Create a CA for signing client certificates
2. Issue client certificates signed by that CA
3. Configure the server with the CA certificate:

```yaml
server:
  tls:
    cert_file: "/path/to/server-cert.pem"
    key_file: "/path/to/server-key.pem"
    client_ca: "/path/to/client-ca.pem"
```

When `client_ca` is set, the server requires and verifies client certificates on every connection.

## Embedding Provider Setup

### OpenAI

```yaml
embedding:
  provider: "openai"
  model: "text-embedding-3-small"
  dimensions: 1536
```

```bash
export INGATAN_EMBEDDING_API_KEY="sk-..."
```

### Ollama

Run an Ollama server with an embedding model:

```bash
ollama pull nomic-embed-text
```

```yaml
embedding:
  provider: "ollama"
  model: "nomic-embed-text"
  dimensions: 768
  base_url: "http://localhost:11434/v1"
```

No API key required for local Ollama.

### Amazon Bedrock

```yaml
embedding:
  provider: "bedrock"
  model: "amazon.titan-embed-text-v2:0"
  dimensions: 1024
```

Uses AWS SDK default credential chain (env vars, `~/.aws/credentials`, IAM role).

## LLM Provider Setup

LLM is used for conversation summarization. It is optional -- without it, `conversation_summarize` returns an error and auto-summarization is disabled.

### Anthropic

```yaml
llm:
  provider: "anthropic"
  model: "claude-3-5-haiku-latest"
```

```bash
export INGATAN_LLM_API_KEY="sk-ant-..."
```

### OpenAI

```yaml
llm:
  provider: "openai"
  model: "gpt-4o-mini"
```

```bash
export INGATAN_LLM_API_KEY="sk-..."
```

### Ollama

```yaml
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434/v1"
```

No API key required for local Ollama.

## Example Minimal Config

For a development setup with OpenAI embeddings:

```yaml
server:
  host: "127.0.0.1"
  port: 8443

data_dir: "~/.ingatan/data"

auth:
  algorithm: "HS256"

embedding:
  provider: "openai"
  model: "text-embedding-3-small"
  dimensions: 1536
```

```bash
export INGATAN_AUTH_SECRET="change-me-to-a-real-secret-at-least-32-chars"
export INGATAN_EMBEDDING_API_KEY="sk-..."
```

```bash
./bin/ingatan --config ~/.ingatan/config.yaml
```
