# Invoice Backend API

Go-based REST API для управления счетами и актами контрагентов.

## Технологии

- Go 1.21+
- Chi Router
- PostgreSQL
- database/sql (без ORM)

## Установка и запуск

### 1. Настройка базы данных

Создайте PostgreSQL базу данных:

```bash
createdb invoices_db
```

Примените миграции:

```bash
psql -d invoices_db -f migrations/001_initial_schema.sql
psql -d invoices_db -f migrations/003_add_document_type.sql
psql -d invoices_db -f migrations/004_add_contract_number.sql
psql -d invoices_db -f migrations/005_add_contract_to_customers.sql
psql -d invoices_db -f migrations/006_repair_contract_number.sql
psql -d invoices_db -f migrations/007_contracts_acts_lines.sql
psql -d invoices_db -f migrations/008_contract_topics.sql
psql -d invoices_db -f migrations/009_archive_flags.sql
psql -d invoices_db -f migrations/010_add_customer_kpp.sql
psql -d invoices_db -f migrations/011_redmine_agent_integration.sql
psql -d invoices_db -f migrations/012_redmine_project_dashboard.sql
psql -d invoices_db -f migrations/013_redmine_dashboard_manager_status.sql
psql -d invoices_db -f migrations/014_redmine_project_operations.sql
psql -d invoices_db -f migrations/015_customer_edo_requisites.sql
psql -d invoices_db -f migrations/016_accounting_mcp.sql
psql -d invoices_db -f migrations/017_dzerzhinskie_vedomosti_kpp.sql
psql -d invoices_db -f migrations/018_fix_default_contract_numbers.sql
psql -d invoices_db -f migrations/019_centr_ttm_director.sql
psql -d invoices_db -f migrations/020_fix_bank_account_and_dz_vedomosti_signer.sql
psql -d invoices_db -f migrations/021_contract_appendices.sql
psql -d invoices_db -f migrations/022_seed_services_catalog.sql
psql -d invoices_db -f migrations/023_dogovor_602_website_appendix.sql
psql -d invoices_db -f migrations/024_fix_duplicate_dz_vedomosti_and_stale_contract_numbers.sql
psql -d invoices_db -f migrations/025_act_invoices_delete_cascade.sql
psql -d invoices_db -f migrations/026_global_unique_act_invoice_numbers.sql
psql -d invoices_db -f migrations/027_fix_dogovor_602_number.sql
psql -d invoices_db -f migrations/028_restore_dogovor_602_appendix.sql
psql -d invoices_db -f migrations/029_zvonari.sql
psql -d invoices_db -f migrations/030_drop_unused_roadmap_milestone_event_type.sql
psql -d invoices_db -f migrations/031_redmine_control_event_notified_at.sql
psql -d invoices_db -f migrations/032_zvonari_engine_tracking.sql
psql -d invoices_db -f migrations/033_zvonari_error_kind.sql
```

Если база была создана через `docker-compose` до этого изменения, существующий volume нужно либо пересоздать, либо применить миграции вручную: init-скрипты Postgres выполняются только при первом создании пустой БД.

### 2. Настройка переменных окружения

Создайте файл `.env` в корне backend директории:

```bash
DATABASE_URL=postgres://username:password@localhost:5432/invoices_db
PORT=8080
CORS_ORIGIN=http://localhost:3000
DADATA_API_KEY=your_dadata_api_key
MCP_AUTH_TOKEN=change-me
```

Для автоподстановки реальных идентификаторов участников ЭДО в XML УПД настройте Saby:

```bash
SABY_BASE_URL=https://online.sbis.ru
SABY_ACCESS_TOKEN=...
# или вместо access token:
SABY_LOGIN=...
SABY_PASSWORD=...
SABY_ACCOUNT_NUMBER=

SABY_SELLER_INN=526220116209
SABY_SELLER_KPP=
SABY_SELLER_NAME=Индивидуальный предприниматель Мыленкова Любовь Валерьевна
```

При скачивании XML акта backend вызовет `СБИС.ИнформацияОКонтрагенте` по ИНН/КПП покупателя и продавца. Найденный ID покупателя кешируется в `customers.edo_id_tensor`; для продавца можно задать `SABY_SELLER_EDO_ID`, чтобы не дергать Saby каждый раз.

### 3. Установка зависимостей

```bash
go mod download
```

### 4. Запуск сервера

```bash
go run cmd/server/main.go
```

Сервер запустится на http://localhost:8080

MCP-сервер бухгалтерских документов:

```bash
MCP_TRANSPORT=http MCP_AUTH_TOKEN=change-me go run cmd/accounting-mcp/main.go
```

По умолчанию MCP endpoint доступен на http://localhost:3000/mcp, healthcheck - на http://localhost:3000/healthz.

## API Endpoints

### Customers (Контрагенты)

- `GET /api/customers` - список всех контрагентов
- `GET /api/customers?search=query` - поиск контрагентов
- `GET /api/customers/{id}` - получить контрагента по ID

### Invoices (Счета)

- `GET /api/invoices?customer_id=xxx` - счета контрагента
- `GET /api/invoices/{id}` - получить счет по ID
- `GET /api/invoices/{id}/services` - счет с услугами
- `POST /api/invoices` - создать счет

**Пример создания счета:**

```json
{
  "customer_id": "uuid",
  "number": "001",
  "date": "13.01.2026",
  "service_ids": ["service-uuid-1", "service-uuid-2"]
}
```

### Services (Услуги)

- `POST /api/services` - создать услугу

**Пример создания услуги:**

```json
{
  "name": "Консультация",
  "price": 5000.00
}
```

## Структура проекта

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Точка входа
├── internal/
│   ├── handlers/                # HTTP handlers
│   ├── models/                  # Структуры данных
│   ├── database/                # Работа с БД
│   └── middleware/              # CORS, logging
├── migrations/                  # SQL миграции
└── go.mod
```

## Разработка

### Добавление новых endpoints

1. Создайте методы в `internal/database/`
2. Создайте handlers в `internal/handlers/`
3. Зарегистрируйте routes в `cmd/server/main.go`

### База данных

Используется чистый SQL без ORM для максимального контроля.
Все запросы находятся в `internal/database/`.

## Деплой

1. Настройте PostgreSQL на production сервере
2. Примените миграции
3. Установите переменные окружения
4. Скомпилируйте бинарник: `go build -o invoices-api cmd/server/main.go`
5. Запустите: `./invoices-api`
