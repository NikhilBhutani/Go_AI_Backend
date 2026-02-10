# BackendWithAI

A domain-agnostic, reusable Go backend framework for AI-powered SaaS applications. Provides foundational components for RAG, fine-tuning, document processing, agents, guardrails, reasoning, evals, and LLM-powered workflows in any vertical.

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
| 2 | **RAG Pipeline** | Ingest → chunk → embed → store in pgvector → retrieve → rerank → generate with citations. Includes HyDE, query rewriting, multi-query retrieval |
| 3 | **Document Processing** | Upload, text extraction (PDF/DOCX/TXT), OCR, async processing via Asynq |
| 4 | **Prompt Management** | Template storage, versioning, `{{variable}}` interpolation, per-tenant overrides |
| 5 | **Fine-tuning Orchestration** | Dataset management, training job submission, model registry |
| 6 | **Multi-Tenancy & Auth** | Supabase JWT validation, RBAC, tenant isolation, API keys |
| 7 | **Job Queue** | Asynq workers for heavy AI tasks, status tracking, retries |
| 8 | **Audit & Observability** | AI call logging, cost aggregation, activity trail, health checks |
| 9 | **Webhook/Event System** | Internal event bus, HMAC-signed webhook delivery with retry |
| 10 | **API Layer** | Chi router, middleware stack (CORS, rate limiting, logging), versioned routes |
| 11 | **Agents** | ReAct pattern, tool calling, prompt chaining, multi-agent orchestration, LLM-based routing |
| 12 | **Guardrails** | Prompt injection detection (heuristic + LLM), content filtering, PII detection, input length validation |
| 13 | **Memory** | BufferMemory (sliding window), TokenWindowMemory (token budget), SummaryMemory (auto-summarization), ContextEngine (token-aware prompt assembly) |
| 14 | **Evals** | LLM-as-Judge, pairwise comparison, relevance scoring, faithfulness evaluation, hallucination detection, claim extraction |
| 15 | **Reasoning** | Chain-of-Thought (zero-shot, few-shot), Tree-of-Thought, Self-Consistency, Reflection (generate-critique-revise), Structured Output |
| 16 | **Multimodal** | Vision analysis (GPT-4o/Claude), image generation (DALL-E), text-to-speech, OCR via vision models |

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
│   │   ├── generator.go             # Context assembly + LLM generation with citations
│   │   ├── reranker.go              # LLM reranker + cross-encoder reranker
│   │   └── query_rewriter.go        # Query rewriting, multi-query, HyDE
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
│   ├── agent/
│   │   ├── agent.go                 # ReAct agent with tool calling + memory
│   │   ├── tools.go                 # Built-in tools (calculator, RAG search, web fetch, JSON extractor)
│   │   ├── react.go                 # ReAct response parser
│   │   └── orchestrator.go          # Multi-agent orchestration, LLM router, prompt chaining
│   ├── guardrails/
│   │   ├── guardrails.go            # Pipeline, PII detector, input length guard
│   │   ├── prompt_injection.go      # Heuristic + LLM-based prompt injection detection
│   │   ├── content_filter.go        # Keyword-based content filtering
│   │   └── intent.go                # LLM-based intent classification
│   ├── memory/
│   │   ├── memory.go                # BufferMemory (sliding window), TokenWindowMemory (token budget)
│   │   ├── summary.go               # SummaryMemory (auto-summarizes via LLM)
│   │   └── context_engine.go        # ContextEngine (assembles prompt within token budgets)
│   ├── eval/
│   │   ├── eval.go                  # EvalSuite, relevance evaluator, faithfulness evaluator
│   │   ├── judge.go                 # LLM-as-Judge (multi-dimensional), pairwise comparison
│   │   └── hallucination.go         # Hallucination detector, claim extraction + verification
│   ├── reasoning/
│   │   ├── cot.go                   # Chain-of-Thought (zero-shot, few-shot)
│   │   ├── tot.go                   # Tree-of-Thought, Self-Consistency
│   │   └── structured.go            # Structured output, Reflection pattern
│   ├── multimodal/
│   │   ├── vision.go                # Vision analysis, image description, OCR, comparison
│   │   └── generation.go            # Image generation (DALL-E), text-to-speech
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
│       └── handlers/                # All API endpoint handlers
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
| `POST` | `/api/v1/rag/query` | RAG query (retrieve + rerank + generate with citations) |
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

### Agents
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/agents/run` | Execute ReAct agent with tools |
| `POST` | `/api/v1/agents/chain` | Execute prompt chain |

### Guardrails
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/guardrails/check` | Validate text against safety checks |
| `POST` | `/api/v1/guardrails/classify` | Intent classification |

### Evals
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/eval/suite` | Run full eval suite (relevance, faithfulness, hallucination, judge) |
| `POST` | `/api/v1/eval/judge` | LLM-as-Judge scoring (accuracy, completeness, clarity, helpfulness) |
| `POST` | `/api/v1/eval/compare` | Pairwise A/B comparison of two responses |

### Reasoning
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/reasoning/cot` | Chain-of-Thought reasoning (zero-shot or few-shot) |
| `POST` | `/api/v1/reasoning/tot` | Tree-of-Thought (explore multiple paths) |
| `POST` | `/api/v1/reasoning/self-consistency` | Self-Consistency (sample multiple, take majority) |
| `POST` | `/api/v1/reasoning/reflect` | Reflection pattern (generate → critique → revise) |

### Multimodal
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/multimodal/vision` | Vision analysis (describe, OCR, compare images) |
| `POST` | `/api/v1/multimodal/image/generate` | Image generation (DALL-E) |
| `POST` | `/api/v1/multimodal/tts` | Text-to-speech |

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

## AI Concepts Covered

This framework implements the following AI/LLM engineering patterns:

### LLM Fundamentals
- Multi-provider gateway (OpenAI, Anthropic, Ollama) with automatic fallback and retry
- Streaming via SSE (Server-Sent Events)
- Token counting and cost tracking per model/provider
- Temperature, top-p, stop sequence configuration

### RAG (Retrieval-Augmented Generation)
- **Chunking**: Fixed-size, recursive, sentence-based strategies
- **Embedding**: Batch embedding via provider APIs
- **Vector Store**: pgvector with IVFFlat indexing
- **Hybrid Search**: Vector similarity + BM25 keyword search
- **Reranking**: LLM-based reranker and cross-encoder reranker
- **Query Rewriting**: Multi-query generation for better recall
- **HyDE**: Hypothetical Document Embeddings — generate a hypothetical answer, embed it, search with that
- **Citations**: Generated answers include source references

### Agents & Tool Use
- **ReAct Pattern**: Thought → Action → Observation loop
- **Tool Calling**: Calculator, RAG search, web fetch, JSON extractor
- **Prompt Chaining**: Sequential LLM calls with output piping
- **Multi-Agent Orchestration**: Multiple specialized agents coordinated by an orchestrator
- **LLM-Based Routing**: Route messages to the best agent via LLM classification

### Memory & Context
- **BufferMemory**: Sliding window of last N messages
- **TokenWindowMemory**: Keep messages within a token budget
- **SummaryMemory**: Auto-summarize older messages via LLM to preserve context
- **ContextEngine**: Assemble system prompt + memory + RAG context within token budgets

### Guardrails & Safety
- **Prompt Injection Detection**: Heuristic pattern matching + LLM-based detection
- **Content Filtering**: Keyword-based harmful content detection
- **PII Detection**: Pattern-based detection of SSNs, credit cards, passwords
- **Input Length Validation**: Configurable max input length
- **Intent Classification**: LLM-based intent detection with configurable intents

### Evaluation
- **LLM-as-Judge**: Multi-dimensional scoring (accuracy, completeness, clarity, helpfulness)
- **Pairwise Comparison**: A/B testing of two responses
- **Relevance Scoring**: How relevant is the response to the query?
- **Faithfulness Evaluation**: Does the response stick to the provided context? (RAG grounding)
- **Hallucination Detection**: Grounded (vs. context) and open-ended (factual) checks
- **Claim Extraction**: Break response into individual claims, verify each one

### Reasoning Patterns
- **Chain-of-Thought (CoT)**: Zero-shot ("Let's think step by step") and few-shot (worked examples)
- **Tree-of-Thought (ToT)**: Explore multiple reasoning paths in parallel, score and pick best
- **Self-Consistency**: Sample multiple chains, take the majority answer
- **Reflection**: Generate → critique → revise loop for iterative improvement
- **Structured Output**: Force LLM responses into defined JSON schemas

### Multimodal
- **Vision Analysis**: Image understanding via GPT-4o / Claude vision models
- **Image Description**: Detailed image captioning
- **OCR via Vision**: Extract text from images using vision models
- **Image Generation**: DALL-E integration for text-to-image
- **Text-to-Speech**: TTS via OpenAI API
- **Multi-Image Comparison**: Compare multiple images on specified aspects

### Fine-tuning
- **Dataset Management**: Upload, validate, format as JSONL
- **Job Orchestration**: Submit training jobs, poll status, track completion
- **Model Registry**: Catalog fine-tuned models with metadata

### Infrastructure
- **Multi-tenancy**: Tenant isolation via JWT + row-level context
- **RBAC**: Role-based access control with permission checking
- **API Key Auth**: SHA-256 hashed API keys with scopes
- **Rate Limiting**: Token bucket algorithm
- **Job Queue**: Asynq workers for document processing, embedding, fine-tuning
- **Webhook Delivery**: HMAC-SHA256 signed payloads with retry
- **Audit Logging**: Track all AI calls, costs, and user actions

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

// Guardrail — implement for custom safety checks
type Guardrail interface {
    Check(ctx context.Context, text string) (*GuardrailResult, error)
    Name() string
}

// Evaluator — implement for custom eval metrics
type Evaluator interface {
    Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error)
    Name() string
}

// Memory — implement for custom memory backends
type Memory interface {
    Add(ctx context.Context, entry Entry)
    Get(ctx context.Context, limit int) []Entry
    Clear(ctx context.Context)
    Size() int
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
