-- Removes the deadlock where deleting an act required unlinking its invoices
-- first, deleting an invoice required unlinking its acts first, and there was
-- no way to unlink either side — an act and an invoice referencing each other
-- via act_invoices could never be deleted.
--
-- act_invoices is a linking/audit table (which invoices contributed to an
-- act); it should never itself block deleting the act or the invoice it
-- links. Switching both FKs from RESTRICT to CASCADE means deleting an act
-- or invoice just removes the stale link row, not the other side.

ALTER TABLE act_invoices
    DROP CONSTRAINT act_invoices_act_id_fkey,
    ADD CONSTRAINT act_invoices_act_id_fkey
        FOREIGN KEY (act_id) REFERENCES acts(id) ON DELETE CASCADE;

ALTER TABLE act_invoices
    DROP CONSTRAINT act_invoices_invoice_id_fkey,
    ADD CONSTRAINT act_invoices_invoice_id_fkey
        FOREIGN KEY (invoice_id) REFERENCES invoices(id) ON DELETE CASCADE;
