-- Reference schema for replacing the durable JSON adapter with PostgreSQL.
CREATE TABLE IF NOT EXISTS wasm_modules (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    digest TEXT NOT NULL,
    metadata_json JSONB NOT NULL,
    content BYTEA NOT NULL,
    state TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, name, version)
);

CREATE INDEX IF NOT EXISTS wasm_modules_tenant_state
    ON wasm_modules (tenant_id, state);

CREATE TABLE IF NOT EXISTS wasm_executions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    module_id TEXT NOT NULL REFERENCES wasm_modules(id),
    request_digest TEXT NOT NULL,
    status TEXT NOT NULL,
    result_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS wasm_executions_tenant_created
    ON wasm_executions (tenant_id, created_at DESC);
