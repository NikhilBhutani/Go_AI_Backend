-- Migration 007: RAPTOR hierarchical indexing & multi-representation support
-- Adds chunk_level, parent_chunk_id, and chunk_type columns to document_chunks.

ALTER TABLE document_chunks
    ADD COLUMN IF NOT EXISTS chunk_level    INT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS parent_chunk_id UUID REFERENCES document_chunks(id),
    ADD COLUMN IF NOT EXISTS chunk_type     TEXT NOT NULL DEFAULT 'standard';

CREATE INDEX IF NOT EXISTS idx_document_chunks_chunk_level      ON document_chunks(chunk_level);
CREATE INDEX IF NOT EXISTS idx_document_chunks_parent_chunk_id  ON document_chunks(parent_chunk_id);
