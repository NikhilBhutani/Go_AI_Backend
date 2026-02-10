CREATE TABLE finetune_datasets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    file_path TEXT,
    record_count INT,
    status TEXT DEFAULT 'draft',
    provider TEXT,
    provider_file_id TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE finetune_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    dataset_id UUID REFERENCES finetune_datasets(id),
    provider TEXT NOT NULL,
    provider_job_id TEXT,
    base_model TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    hyperparams JSONB DEFAULT '{}',
    result_model TEXT,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE model_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model_id TEXT NOT NULL,
    base_model TEXT,
    finetune_job_id UUID REFERENCES finetune_jobs(id),
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now()
);
