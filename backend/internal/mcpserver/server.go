package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"invoices-backend/internal/accounting"
)

type toolFunc[In any] func(context.Context, In) (any, error)

func New(service *accounting.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "hermes-accounting", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Accounting MCP server for Hermes. Use prepare_* before commit_* for all significant write actions. The seller organization uses без НДС.",
	})

	addTool(server, "counterparties.search", "Search counterparties by name, INN, KPP, OGRN, phone, or email.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.SearchCounterparties(ctx, input)
	})
	addTool(server, "counterparties.get", "Get a counterparty by ID.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetCounterparty(ctx, input)
	})
	addTool(server, "counterparties.prepare_create", "Prepare counterparty creation and return a confirmation token.", true, false, true, func(ctx context.Context, input accounting.CreateCounterpartyInput) (any, error) {
		return service.PrepareCreateCounterparty(ctx, input)
	})
	addTool(server, "counterparties.commit_create", "Commit prepared counterparty creation. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitCreateCounterparty(ctx, input)
	})
	addTool(server, "counterparties.prepare_update", "Prepare counterparty update and return a confirmation token.", true, false, true, func(ctx context.Context, input accounting.UpdateCounterpartyInput) (any, error) {
		return service.PrepareUpdateCounterparty(ctx, input)
	})
	addTool(server, "counterparties.commit_update", "Commit prepared counterparty update. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitUpdateCounterparty(ctx, input)
	})
	addTool(server, "counterparties.prepare_archive", "Prepare soft archival of a counterparty.", true, true, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareArchiveCounterparty(ctx, input)
	})
	addTool(server, "counterparties.archive", "Commit soft archival of a counterparty. Requires confirmation_token and idempotency_key.", false, true, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.ArchiveCounterparty(ctx, input)
	})
	addTool(server, "counterparties.list_documents", "List invoices and acts for a counterparty.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.ListCounterpartyDocuments(ctx, input)
	})
	addTool(server, "counterparties.list_contracts", "List contracts for a counterparty.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.ListCounterpartyContracts(ctx, input)
	})

	addTool(server, "contracts.search", "Search contracts by number, subject, counterparty name, or INN.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.SearchContracts(ctx, input)
	})
	addTool(server, "contracts.get", "Get a contract by ID.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetContract(ctx, input)
	})
	addTool(server, "contracts.prepare_create", "Prepare contract creation and return a confirmation token.", true, false, true, func(ctx context.Context, input accounting.CreateContractInput) (any, error) {
		return service.PrepareCreateContract(ctx, input)
	})
	addTool(server, "contracts.commit_create", "Commit prepared contract creation. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitCreateContract(ctx, input)
	})
	addTool(server, "contracts.prepare_archive", "Prepare contract archival.", true, true, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareArchiveContract(ctx, input)
	})
	addTool(server, "contracts.archive", "Commit contract archival. Requires confirmation_token and idempotency_key.", false, true, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.ArchiveContract(ctx, input)
	})
	addTool(server, "contracts.list_documents", "List invoices and acts for a contract.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.ListContractDocuments(ctx, input)
	})

	addTool(server, "services.search", "Search the services catalog (standard price list sections plus any ad-hoc services) by name or section. Use this before contract_appendices.prepare_create to find catalog item IDs.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.SearchServiceCatalog(ctx, input)
	})

	addTool(server, "contract_appendices.prepare_create", "Prepare a new Приложение к договору (contract appendix / смета) — a standalone printable document listing catalog and/or ad-hoc work items for a contract, grouped into sections. Use services.search or a services catalog listing to find catalog items first.", true, false, true, func(ctx context.Context, input accounting.CreateContractAppendixInput) (any, error) {
		return service.PrepareCreateContractAppendix(ctx, input)
	})
	addTool(server, "contract_appendices.commit_create", "Commit a prepared contract appendix. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitCreateContractAppendix(ctx, input)
	})

	addTool(server, "invoices.search", "Search invoices.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.SearchInvoices(ctx, input)
	})
	addTool(server, "invoices.get", "Get an invoice with service lines.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetInvoice(ctx, input)
	})
	addTool(server, "invoices.prepare_create", "Prepare invoice creation. Does not reserve a document number.", true, false, true, func(ctx context.Context, input accounting.CreateInvoiceInput) (any, error) {
		return service.PrepareCreateInvoice(ctx, input)
	})
	addTool(server, "invoices.commit_create", "Commit prepared invoice creation and assign a transactional number.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitCreateInvoice(ctx, input)
	})
	addTool(server, "invoices.prepare_issue", "Prepare invoice issue status change.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareIssueInvoice(ctx, input)
	})
	addTool(server, "invoices.commit_issue", "Commit invoice issue status change.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitIssueInvoice(ctx, input)
	})
	addTool(server, "invoices.prepare_mark_paid", "Prepare manual paid marker for an invoice.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareMarkInvoicePaid(ctx, input)
	})
	addTool(server, "invoices.mark_paid", "Mark an invoice paid manually. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.MarkInvoicePaid(ctx, input)
	})
	addTool(server, "invoices.prepare_cancel", "Prepare invoice cancellation.", true, true, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareCancelInvoice(ctx, input)
	})
	addTool(server, "invoices.cancel", "Cancel an invoice. Requires confirmation_token and idempotency_key.", false, true, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CancelInvoice(ctx, input)
	})
	addTool(server, "invoices.list_unpaid", "List unpaid invoices. Bank confirmation is not inferred.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.ListUnpaidInvoices(ctx, input)
	})
	addTool(server, "invoices.render_pdf", "Render a server-side PDF file for an already-created invoice. Idempotent regeneration of an artifact, not a new business document, so it does not go through prepare/commit.", false, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.RenderPDF(ctx, accounting.RenderFileInput{DocumentType: "invoice", DocumentID: input.ID})
	})
	addTool(server, "invoices.get_file", "Get the latest stored invoice file metadata.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetFile(ctx, accounting.RenderFileInput{DocumentType: "invoice", DocumentID: input.ID})
	})

	addTool(server, "acts.search", "Search acts.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.SearchActs(ctx, input)
	})
	addTool(server, "acts.get", "Get an act with service lines and linked invoices.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetAct(ctx, input)
	})
	addTool(server, "acts.prepare_create", "Prepare act creation, optionally from invoice_id.", true, false, true, func(ctx context.Context, input accounting.CreateActInput) (any, error) {
		return service.PrepareCreateAct(ctx, input)
	})
	addTool(server, "acts.commit_create", "Commit prepared act creation and assign a transactional number.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitCreateAct(ctx, input)
	})
	addTool(server, "acts.create_from_invoice", "Alias for acts.prepare_create with invoice_id; returns a confirmation token.", true, false, true, func(ctx context.Context, input accounting.CreateActInput) (any, error) {
		return service.PrepareCreateAct(ctx, input)
	})
	addTool(server, "acts.prepare_issue", "Prepare act issue/sign status change.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareIssueAct(ctx, input)
	})
	addTool(server, "acts.commit_issue", "Commit act issue/sign status change.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitIssueAct(ctx, input)
	})
	addTool(server, "acts.prepare_cancel", "Prepare act cancellation.", true, true, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.PrepareCancelAct(ctx, input)
	})
	addTool(server, "acts.cancel", "Cancel an act. Requires confirmation_token and idempotency_key.", false, true, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CancelAct(ctx, input)
	})
	addTool(server, "acts.render_pdf", "Render a server-side PDF file for an already-created act. Idempotent regeneration of an artifact, not a new business document, so it does not go through prepare/commit.", false, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.RenderPDF(ctx, accounting.RenderFileInput{DocumentType: "act", DocumentID: input.ID})
	})
	addTool(server, "acts.export_upd_xml", "Generate and store UPD XML 5.03 for an already-created act. Idempotent regeneration of an artifact (same storage path, upserted on each call), not a new business document, so it does not go through prepare/commit.", false, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.ExportActUPDXML(ctx, input)
	})
	addTool(server, "acts.validate_upd_xml", "Validate act UPD XML with parser and internal business checks.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.ValidateActUPDXML(ctx, input)
	})
	addTool(server, "acts.get_file", "Get the latest stored act file metadata.", true, false, true, func(ctx context.Context, input accounting.IDInput) (any, error) {
		return service.GetFile(ctx, accounting.RenderFileInput{DocumentType: "act", DocumentID: input.ID})
	})

	addTool(server, "documents.prepare_update_number", "Prepare changing the numeric document number for an invoice or act. Input: document_type, document_id, number.", true, false, true, func(ctx context.Context, input accounting.UpdateDocumentNumberInput) (any, error) {
		return service.PrepareUpdateDocumentNumber(ctx, input)
	})
	addTool(server, "documents.commit_update_number", "Commit prepared document number change. Requires confirmation_token and idempotency_key.", false, false, true, func(ctx context.Context, input accounting.CommitInput) (any, error) {
		return service.CommitUpdateDocumentNumber(ctx, input)
	})

	addTool(server, "reports.unpaid_invoices", "Report unpaid invoices.", true, false, true, func(ctx context.Context, input accounting.SearchInput) (any, error) {
		return service.ListUnpaidInvoices(ctx, input)
	})

	addResources(server, service)
	addPrompts(server)
	return server
}

func addTool[In any](server *mcp.Server, name, description string, readOnly bool, destructive bool, idempotent bool, fn toolFunc[In]) {
	openWorld := false
	destructiveHint := destructive
	mcp.AddTool[In, map[string]any](server, &mcp.Tool{
		Name:        name,
		Title:       name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    readOnly,
			DestructiveHint: &destructiveHint,
			IdempotentHint:  idempotent,
			OpenWorldHint:   &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, map[string]any, error) {
		value, err := fn(ctx, input)
		if err != nil {
			return nil, map[string]any{"ok": false, "error": errorPayload(err)}, nil
		}
		return nil, map[string]any{"ok": true, "result": value}, nil
	})
}

func addResources(server *mcp.Server, service *accounting.Service) {
	handler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		value, err := service.Resource(ctx, uri)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(data)}},
		}, nil
	}

	server.AddResource(&mcp.Resource{
		URI:         "accounting://organization/current",
		Name:        "current_organization",
		Title:       "Current accounting organization",
		MIMEType:    "application/json",
		Description: "Current seller organization and accounting settings.",
	}, handler)
	server.AddResource(&mcp.Resource{
		URI:         "accounting://reports/unpaid-invoices",
		Name:        "unpaid_invoices",
		Title:       "Unpaid invoices",
		MIMEType:    "application/json",
		Description: "Read-only unpaid invoice report.",
	}, handler)

	for _, tmpl := range []struct {
		uri  string
		name string
		desc string
	}{
		{"accounting://counterparties/{id}", "counterparty", "Counterparty by ID."},
		{"accounting://contracts/{id}", "contract", "Contract by ID."},
		{"accounting://invoices/{id}", "invoice", "Invoice by ID."},
		{"accounting://acts/{id}", "act", "Act by ID."},
		{"accounting://documents/{id}/files", "document_files", "Stored files for a document ID."},
	} {
		server.AddResourceTemplate(&mcp.ResourceTemplate{
			URITemplate: tmpl.uri,
			Name:        tmpl.name,
			MIMEType:    "application/json",
			Description: tmpl.desc,
		}, handler)
	}
}

func addPrompts(server *mcp.Server) {
	prompts := map[string]string{
		"create_monthly_invoice":          "Find the counterparty and active contract, prepare an invoice for the requested month, show preview, ask for confirmation, then commit with an idempotency key.",
		"create_monthly_act":              "Find the counterparty and active contract, prepare an act for the requested month, show preview, ask for confirmation, then commit with an idempotency key and export UPD XML.",
		"create_invoice_and_act":          "Prepare invoice first, ask for confirmation, commit it, then prepare an act from the created invoice and commit it after separate confirmation.",
		"check_document_before_issue":     "Fetch the document, validate required fields, render files if needed, and for acts call acts.validate_upd_xml before suggesting issue/sign.",
		"repeat_previous_month_documents": "Search previous month documents, prepare equivalent documents for the requested period, show all warnings, and require confirmation before each commit or batch.",
		"show_unpaid_invoices":            "Call reports.unpaid_invoices or invoices.list_unpaid and explain that payment status is manual unless bank integration is configured.",
	}
	for name, text := range prompts {
		promptText := text
		server.AddPrompt(&mcp.Prompt{Name: name, Description: promptText}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: promptText,
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: promptText}},
				},
			}, nil
		})
	}
}

// errorPayload turns a Go error into the JSON error shape returned to Hermes.
// Only *accounting.AccountingError carries a message meant for the client;
// anything else is a bug or an unclassified failure (DB outage, driver error,
// etc.) and must never reach the assistant as raw Go/Postgres text — it is
// logged server-side instead and replaced with a safe, generic message.
func errorPayload(err error) map[string]any {
	var app *accounting.AccountingError
	if errors.As(err, &app) {
		if app.UnderlyingError != "" {
			// The client-facing message is intentionally generic (e.g.
			// STORAGE_ERROR) — log the real cause here so it's not lost.
			log.Printf("mcp: %s (client message: %q): %s", app.Code, app.Message, app.UnderlyingError)
		}
		return map[string]any{
			"code":             app.Code,
			"message":          app.Message,
			"details":          app.Details,
			"recoverable":      app.Recoverable,
			"suggested_action": app.SuggestedAction,
		}
	}
	log.Printf("mcp: unclassified internal error: %v", err)
	// Unclassified means it's a bug or an unexpected failure mode the code
	// didn't anticipate — every foreseeable, genuinely transient condition
	// already gets a classified *AccountingError with its own Recoverable
	// value above. Defaulting to recoverable:false here is the safer choice:
	// most unclassified errors (a nil dereference, a bad query, a value that
	// always fails to marshal) will fail identically on retry, and blindly
	// telling the caller to retry an unknown failure risks a retry storm
	// against what might be a systemic outage.
	return map[string]any{
		"code":             "INTERNAL_ERROR",
		"message":          "Внутренняя ошибка сервера.",
		"recoverable":      false,
		"suggested_action": "Обратитесь к администратору",
	}
}
