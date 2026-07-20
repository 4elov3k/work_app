# Hermes CRM Integration Plan

Hermes should work through a small agent-facing API first. MCP can then wrap the
same API without duplicating document logic.

## Goal

Allow Hermes to:

- find customers, contracts, invoices, and acts;
- generate or fetch a document PDF;
- upload the PDF to the CRM system;
- report upload status back to the operator.

## Recommended Shape

### 1. Agent API in this app

Add an `/api/agent` namespace protected by a dedicated bearer token:

- `GET /api/agent/customers?search=...`
- `GET /api/agent/customers/{id}`
- `GET /api/agent/customers/{id}/documents`
- `GET /api/agent/invoices/{id}`
- `GET /api/agent/acts/{id}`
- `GET /api/agent/invoices/{id}/pdf`
- `GET /api/agent/acts/{id}/pdf`
- `POST /api/agent/crm/uploads`

Upload request:

```json
{
  "document_type": "invoice",
  "document_id": "uuid",
  "customer_id": "uuid",
  "crm_entity_id": "crm-contact-or-deal-id",
  "file_name": "Счет_2051_30-01-2026.pdf"
}
```

The backend should generate the PDF server-side or retrieve a stored PDF, then
upload it to CRM with CRM credentials stored only in backend env vars.

### 2. CRM Adapter

Create a backend service boundary:

- `internal/crm/client.go`
- env vars:
  - `CRM_BASE_URL`
  - `CRM_API_TOKEN`
  - `AGENT_API_TOKEN`

The adapter should hide CRM-specific upload details from handlers. Hermes should
never receive the CRM token.

### 3. Audit

Persist every Hermes action:

- actor: `hermes`
- operation: `upload_pdf_to_crm`
- document type and id
- customer id
- CRM entity id
- status and error text
- timestamps

This gives an operator-visible history and makes retries safer.

### 4. MCP Wrapper

After the HTTP API is stable, expose these MCP tools for Hermes:

- `search_customers`
- `list_customer_documents`
- `get_document`
- `upload_document_pdf_to_crm`

Each MCP tool should call the `/api/agent/...` endpoints. MCP stays thin; the
business rules remain in the app backend.

## Why API First

The app already owns customers, contracts, invoices, acts, and document
generation. Keeping CRM upload inside the backend gives one permission model,
one audit trail, and one place for validation. MCP is best used as the agent
interface on top of that, not as the primary business layer.

## Open Inputs Needed

- CRM product name and upload API documentation.
- CRM auth method: bearer token, OAuth, webhook key, or another scheme.
- CRM target object: contact, company, deal, task, or custom entity.
- Whether PDFs should be generated server-side or uploaded from the browser.
