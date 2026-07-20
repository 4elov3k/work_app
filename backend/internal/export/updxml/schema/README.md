# FNS/SBIS UPD XSD

Put the current formalized UPD XML schema package here before enabling strict
validation tests.

Required format:

- Document: UPD / invoice and transfer document for services
- KND: `1115131`
- File prefix: `ON_NSCHFDOPPR`
- Function used by this app: `ДОП`
- Current generator version: `ВерсФорм="5.03"`
- Target check: UPD invoice-form line `5б` / shipment document details

Download checklist:

1. Open `https://format.nalog.ru/` from a Russian IP address without foreign VPN.
2. Search for `1115131` or `ON_NSCHFDOPPR`.
3. Choose the latest active format for an electronic invoice / UPD document.
4. Download the package that contains XSD files, not only PDF/Excel forms.
5. Place the whole extracted package under a versioned folder, for example:
   `backend/internal/export/updxml/schema/fns-2026-04/`.

If using SBIS support instead of FNS, request:

`Актуальная XSD-схема формализованного УПД КНД 1115131, файл ON_NSCHFDOPPR,
Функция=ДОП, действующая с 01.04.2026, включая реквизит строки 5б.`

After adding the schema package, wire tests to validate generated XML against
the exact XSD file referenced by the package.
