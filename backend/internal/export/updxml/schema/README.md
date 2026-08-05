# FNS/SBIS UPD XSD

The official FNS schema effective from 2026-01-01 is stored in
`fns-2026-01/ON_NSCHFDOPPR_1_997_01_05_03_05.xsd`. The act export test validates
generated XML against this schema with `xmllint` when it is available.

- Source: `https://www.nalog.gov.ru/html/sites/www.new.nalog.ru/files/about_fts/docs/xsd/14414412.zip`
- Downloaded: 2026-08-03
- XSD SHA-256: `9bd06a0c6eaffd33e011efc0a7bf96346bcbf14ea1da9b88049c96a30f1b8221`

Required format:

- Document: UPD / invoice and transfer document for services
- KND: `1115131`
- File prefix: `ON_NSCHFDOPPR`
- Default act function for IP Mylenkova: `ДОП`
- Invoice-bearing UPD function (explicit mode only): `СЧФДОП`
- Current generator version: `ВерсФорм="5.03"`
- Target check: no-VAT primary service act and structured contract basis

Update checklist:

1. Open `https://format.nalog.ru/` from a Russian IP address without foreign VPN.
2. Search for `1115131` or `ON_NSCHFDOPPR`.
3. Choose the latest active format for an electronic invoice / UPD document.
4. Download the package that contains XSD files, not only PDF/Excel forms.
5. Place the whole extracted package under a versioned folder, for example:
   `backend/internal/export/updxml/schema/fns-2026-01/`.

Current implementation note:

- ordinary work_app payment invoices are not exported as advance VAT invoices;
- a contract is exported as `СвПродПер/СвПер/ОснПер`; when it is absent,
  `СвПродПер/СвПер/БезДокОснПер` is used;
- the default seller mode is `documentType=act`, `vatMode=none`,
  `issueInvoice=false`, which maps to `Функция=ДОП`;
- `СЧФДОП` is available only through explicit invoice-bearing UPD options;
- shipment document line `5а` is exported as `ДокПодтвОтгрНом`;
- payment document line `5` is not populated from ordinary work_app invoices,
  because an invoice for payment is not a bank payment document.

If using SBIS support instead of FNS, request:

`Актуальная XSD-схема формализованного УПД КНД 1115131, файл ON_NSCHFDOPPR,
Функция=ДОП/СЧФДОП, действующая с 01.01.2026.`

When FNS publishes a newer revision, replace the versioned schema and update the
test path only after checking the revision's effective date.
