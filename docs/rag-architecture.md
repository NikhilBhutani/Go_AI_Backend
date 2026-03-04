# RAG Architecture

This document describes the production-grade RAG system in `internal/rag/` and supporting packages.

---

## Overview: The 4 Pillars

| Pillar | Implementation |
|---|---|
| **Query Construction** | `retriever.go` — vector + hybrid (BM25) search via pgvector |
| **Intelligent Routing** | `router.go` — LLM query classifier + `decomposer.go` for complex queries |
| **Advanced Indexing** | `indexing/raptor.go`, `indexing/multi_rep.go`, `pkg/chunker/semantic.go` |
| **Evaluation** | `internal/eval/` — relevance, faithfulness, hallucination, LLM-judge |

---

## Query Pipeline

```
User Query
    │
    ▼
QueryRouter.Route()          ← classifies: simple / complex / comparison
    │
    ├─ simple ──────────────────► Retriever (vector or hybrid)
    │
    ├─ complex ──► Decomposer ──► N × Retriever ──► RRF Fusion
    │
    └─ comparison ──► QueryRewriter ──► N × Retriever ──► RRF Fusion
                                            │
                                      Reranker (optional)
                                            │
                                      Generator (with citations)
                                            │
                                      QueryResponse { answer, citations, routing_info }
```

### Query Routing (`router.go`)

`LLMQueryRouter` asks a small LLM to classify each query into one of three strategies:

| Strategy | When | Effect |
|---|---|---|
| `simple` | Factual single-hop | Standard vector/hybrid retrieval |
| `complex` | Multi-part analytical | Decompose into sub-questions, retrieve each, RRF merge |
| `comparison` | Comparing entities | Multi-query rewriting, RRF merge |

The `QueryResponse` includes `routing_info` so callers can observe the routing decision.

Callers can override routing by setting `QueryRequest.Strategy`.

### Query Decomposition (`decomposer.go`)

For `complex` queries, `LLMDecomposer` splits the question into 2–4 focused sub-questions.
Each sub-question is retrieved independently and results are fused with RRF.

### Reciprocal Rank Fusion (`fusion.go`)

RRF combines multiple ranked result lists without needing calibrated scores:

```
score(d) = Σ 1 / (k + rank_i(d))   k = 60
```

Used by both decomposition and multi-query rewriting.

---

## Ingestion Pipeline

```
Document
    │
    ├─ strategy: "recursive"  ──► RecursiveChunker
    ├─ strategy: "sentence"   ──► SentenceChunker
    ├─ strategy: "fixed"      ──► FixedChunker
    └─ strategy: "semantic"   ──► SemanticChunker (embedding-based)
                    │
                    ▼
              index_type: "standard"  ──► Embed chunks ──► Store
              index_type: "raptor"    ──► RaptorIndexer (cluster+summarise, multi-level)
              index_type: "multi_rep" ──► MultiRepIndexer (summarise→embed, store full)
```

### Semantic Chunking (`pkg/chunker/semantic.go`)

Splits text at topic boundaries by comparing cosine similarity between adjacent sentence-window embeddings. A split is inserted wherever similarity drops below `threshold` (default 0.8).

Parameters: `threshold float64`, `bufferSize int` (sentences per window, default 3).

### RAPTOR Hierarchical Indexing (`indexing/raptor.go`)

1. Stores leaf chunks at `chunk_level = 0`.
2. Clusters leaves by cosine similarity into groups of `clusterSize` (default 5).
3. Summarises each cluster with an LLM → `chunk_level = 1`.
4. Repeats until one cluster remains (root).
5. All levels are stored in `document_chunks`, enabling retrieval at any abstraction level.

### Multi-Representation Indexing (`indexing/multi_rep.go`)

For each chunk:
- Generates a 1–2 sentence LLM summary.
- Embeds the **summary** (search vector — better semantic precision).
- Stores the **full original content** (retrieval content — richer context for generation).

Chunks are tagged `chunk_type = "multi_rep"` in metadata.

---

## DB Schema (`migrations/007_raptor_multi_rep.sql`)

```sql
ALTER TABLE document_chunks
    ADD COLUMN chunk_level     INT  NOT NULL DEFAULT 0,
    ADD COLUMN parent_chunk_id UUID REFERENCES document_chunks(id),
    ADD COLUMN chunk_type      TEXT NOT NULL DEFAULT 'standard';
```

Indexes on `chunk_level` and `parent_chunk_id` support efficient level-based and tree-walk queries.

---

## API Examples

### Query with automatic routing

```json
POST /api/v1/rag/query
{
  "query": "Compare the pros and cons of transformer vs CNN architectures",
  "top_k": 8,
  "hybrid": true,
  "rerank": true
}
```

Response includes `routing_info`:
```json
{
  "answer": "...",
  "citations": [...],
  "routing_info": { "strategy": "comparison", "reasoning": "comparing two architectures" }
}
```

### Ingest with RAPTOR

```json
POST /api/v1/rag/ingest
{
  "document_id": "...",
  "content": "...",
  "chunk_opts": { "strategy": "recursive", "chunk_size": 512 },
  "index_type": "raptor"
}
```

### Ingest with semantic chunking + multi-representation

```json
POST /api/v1/rag/ingest
{
  "document_id": "...",
  "content": "...",
  "chunk_opts": { "strategy": "semantic" },
  "index_type": "multi_rep"
}
```

---

## Component Reference

| File | Purpose |
|---|---|
| `pipeline.go` | Orchestrator — Ingest, Query, Search |
| `router.go` | LLM-based query classifier |
| `decomposer.go` | Complex query → sub-questions |
| `fusion.go` | Reciprocal Rank Fusion |
| `retriever.go` | Vector + hybrid retrieval |
| `query_rewriter.go` | Multi-query rewriting + HyDE |
| `reranker.go` | LLM reranker + cross-encoder |
| `generator.go` | Answer generation with citations |
| `chunker.go` | Chunking bridge (incl. semantic) |
| `indexing/raptor.go` | RAPTOR hierarchical indexer |
| `indexing/multi_rep.go` | Multi-representation indexer |
| `pkg/chunker/chunker.go` | Fixed / recursive / sentence chunking |
| `pkg/chunker/semantic.go` | Embedding-based semantic chunking |
