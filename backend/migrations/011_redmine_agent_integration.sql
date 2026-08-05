-- Redmine links, document sync statuses, and Hermes audit trail.

BEGIN;

CREATE TABLE IF NOT EXISTS external_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    local_entity_type TEXT NOT NULL,
    local_entity_id UUID NOT NULL,
    system TEXT NOT NULL DEFAULT 'redmine',
    external_entity_type TEXT NOT NULL,
    external_id TEXT NOT NULL,
    external_identifier TEXT,
    external_name TEXT,
    external_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_primary BOOLEAN NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT external_links_local_entity_type_check CHECK (
        local_entity_type IN ('customer','contract','invoice','act','invoice_line','act_line')
    ),
    CONSTRAINT external_links_system_check CHECK (system IN ('redmine')),
    CONSTRAINT external_links_external_entity_type_check CHECK (
        external_entity_type IN ('project','company','contact','issue','attachment','file','custom_field','category')
    )
);

CREATE INDEX IF NOT EXISTS idx_external_links_local
    ON external_links(local_entity_type, local_entity_id);
CREATE INDEX IF NOT EXISTS idx_external_links_external
    ON external_links(system, external_entity_type, external_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_external_links_one_primary_customer_project
    ON external_links(local_entity_type, local_entity_id, system, external_entity_type)
    WHERE is_primary = TRUE AND local_entity_type = 'customer' AND external_entity_type = 'project';

CREATE TABLE IF NOT EXISTS redmine_document_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_type TEXT NOT NULL,
    document_id UUID NOT NULL,
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    redmine_project_id TEXT NOT NULL,
    redmine_project_identifier TEXT,
    redmine_project_name TEXT,
    redmine_file_id TEXT,
    redmine_attachment_id TEXT,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/pdf',
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    uploaded_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT redmine_document_uploads_document_type_check CHECK (document_type IN ('invoice','act')),
    CONSTRAINT redmine_document_uploads_status_check CHECK (
        status IN ('pending','uploaded','failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_redmine_document_uploads_document
    ON redmine_document_uploads(document_type, document_id);
CREATE INDEX IF NOT EXISTS idx_redmine_document_uploads_customer
    ON redmine_document_uploads(customer_id);

CREATE TABLE IF NOT EXISTS agent_pending_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor TEXT NOT NULL DEFAULT 'hermes',
    operation TEXT NOT NULL,
    entity_type TEXT,
    entity_id UUID,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    preview JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL '15 minutes'),
    confirmed_at TIMESTAMP WITH TIME ZONE,
    canceled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT agent_pending_actions_status_check CHECK (
        status IN ('pending','confirmed','canceled','expired')
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_pending_actions_status
    ON agent_pending_actions(status, expires_at);

CREATE TABLE IF NOT EXISTS agent_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor TEXT NOT NULL DEFAULT 'hermes',
    operation TEXT NOT NULL,
    entity_type TEXT,
    entity_id UUID,
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    confirmation_id UUID REFERENCES agent_pending_actions(id) ON DELETE SET NULL,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT agent_audit_logs_status_check CHECK (
        status IN ('success','failed','pending','canceled')
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_entity
    ON agent_audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_customer
    ON agent_audit_logs(customer_id);
CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_created_at
    ON agent_audit_logs(created_at DESC);

CREATE TRIGGER update_external_links_updated_at BEFORE UPDATE ON external_links
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_redmine_document_uploads_updated_at BEFORE UPDATE ON redmine_document_uploads
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_pending_actions_updated_at BEFORE UPDATE ON agent_pending_actions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
