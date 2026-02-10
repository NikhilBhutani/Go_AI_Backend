# BackendWithAI

A domain-agnostic, reusable Go backend framework for AI-powered SaaS applications. Provides foundational components for RAG, fine-tuning, document processing, and LLM-powered workflows in any vertical.

## Tech Stack

- **Go** + Chi router
- **PostgreSQL** via Supabase + pgvector
- **Redis** + Asynq (job queue + cache)
- **LLM Providers:** OpenAI, Anthropic, Ollama
- **Supabase** Auth + Storage

## Components

| # | Component | Description |
|---|-----------|-------------|
| 1 | **LLM Gateway** | Multi-provider abstraction (OpenAI, Anthropic, Ollama), streaming, retry/fallback, cost tracking |
| 2 | **RAG Pipeline** | Ingest → chunk → embed → store in pgvector → retrieve → generate with citations |
| 3 | **Document Processing** | Upload, text extraction (PDF/DOCX/TXT), OCR, async processing via Asynq |
| 4 | **Prompt Management** | Template storage, versioning, `{{variable}}` interpolation, per-tenant overrides |
| 5 | **Fine-tuning Orchestration** | Dataset management, training job submission, model registry |
| 6 | **Multi-Tenancy & Auth** | Supabase JWT validation, RBAC, tenant isolation, API keys |
| 7 | **Job Queue** | Asynq workers for heavy AI tasks, status tracking, retries |
| 8 | **Audit & Observability** | AI call logging, cost aggregation, activity trail, health checks |
| 9 | **Webhook/Event System** | Internal event bus, HMAC-signed webhook delivery with retry |
| 10 | **API Layer** | Chi router, middleware stack (CORS, rate limiting, logging), versioned routes |

## Quick Start

```bash
# 1. Clone and configure
cp .env.example .env   # fill in your API keys and database URL

# 2. Start infrastructure
make docker-up         # starts Redis + PostgreSQL (with pgvector)

# 3. Run the API server
make run               # go run cmd/api/main.go

# 4. Run the worker (separate terminal)
make run-worker        # go run cmd/worker/main.go

# 5. Verify
curl http://localhost:8080/healthz
```

## Project Structure

```
BackendWithAI/
├── cmd/
│   ├── api/main.go                  # HTTP API server entrypoint
│   └── worker/main.go               # Asynq worker entrypoint
├── internal/
│   ├── config/config.go             # Env-based configuration
│   ├── database/
│   │   ├── database.go              # PostgreSQL connection pool (pgxpool)
│   │   └── migrations.go            # Auto-migration runner
│   ├── models/                      # DB models (tenant, user, document, prompt, finetune, audit)
│   ├── auth/
│   │   ├── middleware.go            # Supabase JWT validation
│   │   ├── rbac.go                  # Role-based access control
│   │   └── apikey.go                # API key authentication
│   ├── tenant/
│   │   ├── context.go               # Tenant/user context helpers
│   │   └── service.go               # Tenant CRUD
│   ├── llm/
│   │   ├── provider.go              # Provider interface + types
│   │   ├── gateway.go               # Multi-provider gateway with fallback + retry
│   │   ├── openai.go                # OpenAI provider
│   │   ├── anthropic.go             # Anthropic/Claude provider
│   │   ├── ollama.go                # Ollama local provider
│   │   └── cost.go                  # Token counting + cost calculation
│   ├── rag/
│   │   ├── pipeline.go              # Full RAG orchestration (ingest + query)
│   │   ├── chunker.go               # Text chunking integration
│   │   ├── retriever.go             # Vector + keyword hybrid retrieval
│   │   └── generator.go             # Context assembly + LLM generation with citations
│   ├── embedding/service.go         # Embedding generation via LLM gateway
│   ├── vectorstore/
│   │   ├── store.go                 # VectorStore interface
│   │   └── pgvector.go              # pgvector implementation (hybrid search)
│   ├── document/
│   │   ├── service.go               # Document upload + CRUD
│   │   ├── extractor.go             # Text extraction orchestration
│   │   └── ocr.go                   # Tesseract OCR
│   ├── prompt/
│   │   ├── service.go               # CRUD + versioning
│   │   └── template.go              # Template rendering with {{variables}}
│   ├── finetune/
│   │   ├── service.go               # Job submission + status
│   │   ├── dataset.go               # JSONL validation + formatting
│   │   └── registry.go              # Model registry
│   ├── storage/supabase.go          # Supabase Storage client
│   ├── queue/
│   │   ├── client.go                # Asynq client wrapper
│   │   ├── handlers.go              # Handler registry
│   │   ├── tasks.go                 # Task type definitions
│   │   └── workers/                 # Document, embedding, finetune workers
│   ├── audit/service.go             # Audit + LLM usage logging
│   ├── webhook/
│   │   ├── service.go               # Webhook registration + dispatch
│   │   └── dispatcher.go            # Async delivery with HMAC signing
│   ├── cache/redis.go               # Redis cache layer
│   └── api/
│       ├── router.go                # Chi router + full route wiring
│       ├── middleware/               # Rate limiting, CORS, request logging
│       └── handlers/                # Health, LLM, RAG, documents, prompts, finetune, webhooks, admin
├── pkg/
│   ├── chunker/chunker.go           # Chunking strategies (fixed, recursive, sentence)
│   ├── tokenizer/tokenizer.go       # Token counting
│   └── textextract/extract.go       # PDF, DOCX, TXT extraction
├── migrations/                      # SQL migration files (001-006)
├── docker-compose.yml               # Redis + PostgreSQL (pgvector) for local dev
├── Dockerfile                       # Multi-stage build
├── Makefile
├── .env.example
├── go.mod
└── go.sum
```

## API Endpoints

### Health & System
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check |
| `GET` | `/readyz` | Readiness (DB + Redis connected) |

### LLM Gateway
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/llm/chat` | Chat completion |
| `POST` | `/api/v1/llm/chat/stream` | Streaming chat (SSE) |
| `POST` | `/api/v1/llm/embed` | Generate embeddings |
| `GET` | `/api/v1/llm/models` | List available models |

### Documents
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/documents` | Upload document (multipart) |
| `GET` | `/api/v1/documents` | List documents |
| `GET` | `/api/v1/documents/:id` | Get document details |
| `DELETE` | `/api/v1/documents/:id` | Delete document |
| `GET` | `/api/v1/documents/:id/status` | Processing status |

### RAG
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/rag/query` | RAG query (retrieve + generate with citations) |
| `POST` | `/api/v1/rag/search` | Vector search only (no generation) |

### Prompts
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/prompts` | Create prompt template |
| `GET` | `/api/v1/prompts` | List prompts |
| `GET` | `/api/v1/prompts/:id` | Get prompt with versions |
| `PUT` | `/api/v1/prompts/:id` | Create new version |
| `POST` | `/api/v1/prompts/:id/render` | Render template with variables |

### Fine-tuning
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/finetune/datasets` | Upload training dataset |
| `GET` | `/api/v1/finetune/datasets` | List datasets |
| `POST` | `/api/v1/finetune/jobs` | Start fine-tuning job |
| `GET` | `/api/v1/finetune/jobs` | List jobs |
| `GET` | `/api/v1/finetune/jobs/:id` | Job status |
| `GET` | `/api/v1/finetune/models` | List fine-tuned models |

### Webhooks
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/webhooks` | Register webhook |
| `GET` | `/api/v1/webhooks` | List webhooks |
| `DELETE` | `/api/v1/webhooks/:id` | Delete webhook |

### Admin
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/admin/usage` | Cost dashboard data |
| `GET` | `/api/v1/admin/audit` | Audit logs |

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `SUPABASE_JWT_SECRET` | Yes | JWT secret for token validation |
| `OPENAI_API_KEY` | No | OpenAI API key |
| `ANTHROPIC_API_KEY` | No | Anthropic API key |
| `OLLAMA_URL` | No | Ollama base URL (default: `http://localhost:11434`) |
| `REDIS_ADDR` | No | Redis address (default: `localhost:6379`) |
| `SUPABASE_URL` | No | Supabase project URL |
| `SUPABASE_SERVICE_KEY` | No | Supabase service role key (for storage) |

## Database

Migrations are applied automatically on startup. To apply manually:

```bash
make migrate
```

6 migration files create tables for: tenants, users, roles, API keys, documents, document chunks (with pgvector), prompts, prompt versions, finetune datasets/jobs, model registry, LLM usage logs, audit logs, webhooks, and webhook deliveries.

## Key Interfaces

```go
// LLM Provider — implement to add new providers
type Provider interface {
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
    Name() string
    Models() []string
}

// Vector Store — implement for alternative vector DBs
type VectorStore interface {
    Upsert(ctx context.Context, chunks []Chunk) error
    SimilaritySearch(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error)
    HybridSearch(ctx context.Context, query string, queryVec []float32, opts SearchOptions) ([]SearchResult, error)
    Delete(ctx context.Context, filter DeleteFilter) error
}

// RAG Pipeline
type Pipeline interface {
    Ingest(ctx context.Context, doc IngestRequest) error
    Query(ctx context.Context, req QueryRequest) (*QueryResponse, error)
    Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
}

// Chunker — implement for custom chunking strategies
type Chunker interface {
    Chunk(text string, opts ChunkOptions) []TextChunk
}
```

## Makefile Commands

```bash
make build        # Build API server and worker binaries
make run          # Run API server
make run-worker   # Run Asynq worker
make dev          # Start Docker services + run API server
make test         # Run tests
make lint         # Run go vet
make migrate      # Apply SQL migrations
make docker-up    # Start Redis + PostgreSQL
make docker-down  # Stop Docker services
make tidy         # go mod tidy
```

## License

MIT
