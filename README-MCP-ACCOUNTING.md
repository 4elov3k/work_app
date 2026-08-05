# Hermes Accounting MCP

## Архитектура

`accounting-mcp` - отдельный Go-сервис в Docker. Он поднимает MCP endpoint и вызывает сервисный слой `backend/internal/accounting`, который работает с существующей PostgreSQL-схемой Hermes: `customers`, `contracts`, `invoices`, `invoice_lines`, `acts`, `act_lines`, `act_invoices`.

Текущий поток:

```text
Hermes
  -> MCP Streamable HTTP
  -> accounting-mcp
  -> internal/accounting.Service
  -> PostgreSQL / document storage / existing UPD XML generator
```

MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.6.1`.

Выбрана стабильная версия SDK без pre-release. По официальной таблице совместимости Go SDK v1.4.0-v1.6.1 поддерживает MCP `2025-11-25` и совместим с `2025-06-18`; transport Streamable HTTP используется на `/mcp`.

## Запуск

```bash
docker compose up --build postgres backend accounting-mcp
```

Healthcheck:

```bash
curl http://localhost:3000/healthz
```

По умолчанию порт MCP наружу не публикуется. Внутри Docker-сети Hermes должен подключаться к:

```text
http://accounting-mcp:3000/mcp
```

Если нужен stdio:

```bash
cd backend
DATABASE_URL='postgres://...' MCP_TRANSPORT=stdio go run ./cmd/accounting-mcp
```

## Переменные

См. `.env.example` и `backend/env.example`.

Основные:

```env
MCP_TRANSPORT=http
MCP_HOST=0.0.0.0
MCP_PORT=3000
MCP_PATH=/mcp
MCP_AUTH_ENABLED=true
MCP_AUTH_TOKEN=
DOCUMENT_STORAGE_PATH=/data/documents
CONFIRMATION_TOKEN_TTL=15m
COUNTERPARTY_LOOKUP_PROVIDER=none
EDI_PROVIDER=none
ALLOW_DOCUMENT_SENDING=false
```

Не храните реальные токены в репозитории.

## Авторизация

HTTP transport защищен заголовком:

```http
Authorization: Bearer <MCP_AUTH_TOKEN>
```

Сравнение токена выполняется constant-time. Входящий MCP-токен не передается во внешние системы и не логируется. Для stdio режима транспортная авторизация не применяется, так как процесс запускается локальным клиентом.

## Подтверждения

Значимые действия идут в два этапа:

1. `prepare_*` валидирует вход, строит preview, не присваивает номер и возвращает `confirmation_token`.
2. `commit_*` принимает `confirmation_token` и `idempotency_key`, проверяет срок жизни, пользователя, действие и hash payload.

Токен одноразовый. Повторный commit с тем же `idempotency_key` и теми же данными возвращает сохраненный результат; тот же ключ с другими данными отклоняется.

## Нумерация

Для MCP используется таблица `accounting_number_sequences`. Номер присваивается только на commit внутри транзакции через `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`, без `MAX(number)+1` как финального механизма.

Старые REST endpoints пока сохраняют прежнее поведение.

## Хранение Файлов

Файлы сохраняются только сервером внутри `DOCUMENT_STORAGE_PATH`:

```text
/data/documents/{organization_id}/{year}/{month}/{document_type}/{document_id}/
```

Клиент не передает произвольный путь. Имя нормализуется через `filepath.Base`, абсолютные пути и `../` не используются.

## MCP Tools

Контрагенты:

```text
counterparties.search
counterparties.get
counterparties.prepare_create
counterparties.commit_create
counterparties.prepare_update
counterparties.commit_update
counterparties.prepare_archive
counterparties.archive
counterparties.list_documents
counterparties.list_contracts
```

Договоры:

```text
contracts.search
contracts.get
contracts.prepare_create
contracts.commit_create
contracts.prepare_archive
contracts.archive
contracts.list_documents
```

Счета:

```text
invoices.search
invoices.get
invoices.prepare_create
invoices.commit_create
invoices.prepare_issue
invoices.commit_issue
invoices.prepare_mark_paid
invoices.mark_paid
invoices.prepare_cancel
invoices.cancel
invoices.list_unpaid
invoices.render_pdf
invoices.get_file
```

Акты:

```text
acts.search
acts.get
acts.prepare_create
acts.commit_create
acts.create_from_invoice
acts.prepare_issue
acts.commit_issue
acts.prepare_cancel
acts.cancel
acts.render_pdf
acts.export_upd_xml
acts.validate_upd_xml
acts.get_file
```

Документы:

```text
documents.prepare_update_number
documents.commit_update_number
```

Отчеты:

```text
reports.unpaid_invoices
```

## MCP Resources

```text
accounting://organization/current
accounting://counterparties/{id}
accounting://contracts/{id}
accounting://invoices/{id}
accounting://acts/{id}
accounting://documents/{id}/files
accounting://reports/unpaid-invoices
```

## MCP Prompts

```text
create_monthly_invoice
create_monthly_act
create_invoice_and_act
check_document_before_issue
repeat_previous_month_documents
show_unpaid_invoices
```

## Контакты Контрагента

`counterparties.search` и `counterparties.get` возвращают контактные поля карточки: `phone`, `email`, `contact_person`, `contact_position`, `comment`, `status`.

Если пользователь просит указать директора, Hermes должен передавать:

```json
{
  "contact_person": "ФИО директора",
  "contact_position": "Директор"
}
```

через `counterparties.prepare_create` или `counterparties.prepare_update`, затем выполнять соответствующий `commit_*` после подтверждения.

## Изменение Номера Документа

Для счета или акта:

1. Вызвать `documents.prepare_update_number`.

```json
{
  "document_type": "invoice",
  "document_id": "<invoice-or-act-id>",
  "number": "2418"
}
```

2. Показать preview и warnings.
3. После подтверждения вызвать `documents.commit_update_number`.

Номер должен содержать только цифры. MCP повторно проверяет уникальность номера в рамках договора на commit. Если новый номер выше текущей последовательности, sequence поднимается до этого номера, чтобы следующий автономер не конфликтовал.

## Пример Hermes

Фактический формат конфигурации Hermes в этом репозитории не найден. Не выдумывая формат, параметры подключения такие:

```text
name: hermes-accounting
transport: streamable_http
url: http://accounting-mcp:3000/mcp
headers:
  Authorization: Bearer ${MCP_AUTH_TOKEN}
```

## Ограничения

PDF сейчас генерируется серверным минимальным PDF-рендерером. Он нужен для открываемого файла из MCP; полноценная печатная форма должна быть вынесена из текущих React-шаблонов в backend-сервис отдельной итерацией.

UPD XML использует существующий `backend/internal/export/updxml` генератор. Runtime validation выполняет XML parser и бизнес-проверки. Строгая XSD-проверка уже есть в Go tests через `xmllint`, когда он установлен.

Реквизиты основной организации добавлены в таблицу `organizations`, но существующий UPD XML генератор пока еще содержит продавца в константах. Это технический долг текущей итерации.

Интеграция со СБИС и отправка документов не реализованы. `EDI_PROVIDER=none`, `ALLOW_DOCUMENT_SENDING=false`; сервер не сообщает об отправке без фактического API-ответа.

## Миграции И Резервное Копирование

Новая миграция:

```text
backend/migrations/016_accounting_mcp.sql
```

Перед применением в production сделайте backup PostgreSQL и каталога документов.

Для существующего docker volume init-скрипты Postgres не применяются автоматически. Выполните миграцию вручную:

```bash
docker compose exec -T postgres psql -U invoices_user -d invoices_db < backend/migrations/016_accounting_mcp.sql
```

## Диагностика

Проверить health:

```bash
docker compose exec accounting-mcp wget -qO- http://localhost:3000/healthz
```

Проверить доступность tools через MCP client с заголовком `Authorization`.

Если `healthz` возвращает `503`, проверьте `DATABASE_URL` и наличие миграции `016_accounting_mcp.sql`.

## Проверка Разработчика

```bash
cd backend
env GOCACHE="$PWD/.gocache" go test ./...
env GOCACHE="$PWD/.gocache" go build ./cmd/accounting-mcp
```
