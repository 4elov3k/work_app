-- "Подписать" button on act/invoice print forms: stamps the seller's seal +
-- signature image onto the printed "Исполнитель" block. Deliberately a
-- separate flag from acts.status/invoices.status — status="signed" on an
-- act already has a distinct meaning (accounting "commit_issue"/posting
-- flow in backend/internal/accounting/service.go), and conflating that with
-- "a seal image is shown on the PDF" would let a cosmetic print action
-- silently flip the act's accounting state.

ALTER TABLE acts
    ADD COLUMN signed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN signed_at TIMESTAMPTZ;

ALTER TABLE invoices
    ADD COLUMN signed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN signed_at TIMESTAMPTZ;
