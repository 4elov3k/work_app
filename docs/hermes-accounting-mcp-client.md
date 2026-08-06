# Hermes -> Accounting MCP

Этот документ описывает, что нужно сделать в репозитории Hermes, чтобы дергать бухгалтерский MCP-сервер `accounting-mcp`.

## Endpoint

В Docker-сети сервис доступен по внутреннему адресу:

```text
http://accounting-mcp:3000/mcp
```

Transport:

```text
Streamable HTTP
```

Если окружение Hermes поддерживает только stdio, сервер можно запустить в stdio-режиме:

```bash
MCP_TRANSPORT=stdio DATABASE_URL='postgres://...' ./app
```

## Env В Hermes

Добавить переменные окружения Hermes:

```env
ACCOUNTING_MCP_URL=http://accounting-mcp:3000/mcp
ACCOUNTING_MCP_AUTH_TOKEN=
```

`ACCOUNTING_MCP_AUTH_TOKEN` должен совпадать с `MCP_AUTH_TOKEN` в контейнере `accounting-mcp`.

## Docker Compose

Hermes должен быть в одной Docker-сети с `accounting-mcp`.

Минимально:

```yaml
services:
  hermes:
    environment:
      ACCOUNTING_MCP_URL: http://accounting-mcp:3000/mcp
      ACCOUNTING_MCP_AUTH_TOKEN: ${MCP_AUTH_TOKEN}
    depends_on:
      accounting-mcp:
        condition: service_healthy
    networks:
      - app-network

  accounting-mcp:
    # сервис объявлен в work_app/docker-compose.yml
```

Порт `3000` наружу публиковать не нужно.

## HTTP Headers

Все MCP HTTP-запросы Hermes должен отправлять с bearer token:

```http
Authorization: Bearer ${ACCOUNTING_MCP_AUTH_TOKEN}
Content-Type: application/json
```

Опционально можно передавать пользователя:

```http
X-Hermes-User: <stable-user-id>
```

Этот user id привязывается к confirmation token. Если Hermes не передает заголовок, используется `hermes`.

## Важное Правило Вызова

Все значимые изменения выполняются в два шага:

1. Вызвать `prepare_*`.
2. Показать пользователю preview и warnings.
3. Получить явное подтверждение.
4. Вызвать `commit_*` с:

```json
{
  "confirmation_token": "...",
  "idempotency_key": "stable-operation-key"
}
```

Не вызывать `commit_*` без подтверждения пользователя.

## Типовые Сценарии

### Поиск Контрагента

```text
counterparties.search
```

Input:

```json
{
  "query": "ЦентрТТМ",
  "limit": 10
}
```

### Создание Контрагента

1. `counterparties.prepare_create`

```json
{
  "type": "legal_entity",
  "name": "ООО ЦентрТТМ",
  "fullname": "Общество с ограниченной ответственностью ЦентрТТМ",
  "inn": "5257120323",
  "kpp": "525701001",
  "address": "..."
}
```

2. Показать `preview`, `warnings`, запросить подтверждение.
3. `counterparties.commit_create`

```json
{
  "confirmation_token": "<token-from-prepare>",
  "idempotency_key": "create-counterparty-5257120323-525701001"
}
```

Контактные данные директора передаются и читаются из полей:

```json
{
  "contact_person": "ФИО директора",
  "contact_position": "Директор",
  "phone": "...",
  "email": "..."
}
```

`counterparties.search` и `counterparties.get` возвращают эти поля, поэтому Hermes должен использовать их как источник данных о директоре, а не пытаться брать директора из названия контрагента.

### Изменение Номера Документа

1. `documents.prepare_update_number`

```json
{
  "document_type": "act",
  "document_id": "<act-id>",
  "number": "2169"
}
```

2. Показать пользователю `preview` и `warnings`.
3. `documents.commit_update_number`

```json
{
  "confirmation_token": "<token-from-prepare>",
  "idempotency_key": "update-act-number-<act-id>-2169"
}
```

`document_type` принимает `invoice` или `act`. Номер должен быть числовым и уникальным в рамках договора.

Если найден дубль, MCP вернет предупреждение и не создаст карточку без явного `allow_duplicate=true`.

### Создание Счета

1. Найти контрагента: `counterparties.search`.
2. Найти договор: `contracts.search` или передать `contract_id`.
3. `invoices.prepare_create`

```json
{
  "counterparty_id": "<customer-id>",
  "contract_id": "<contract-id>",
  "date": "31.08.2026",
  "lines": [
    {
      "title": "Услуги по поисковому продвижению сайта за август 2026 года",
      "unit": "шт",
      "qty": "1",
      "price": "24900.00"
    }
  ]
}
```

4. Показать preview: покупатель, договор, строки, сумма, `НДС: без НДС`.
5. После подтверждения вызвать `invoices.commit_create`.
6. Для PDF вызвать `invoices.render_pdf`.

### Создание Акта По Счету

1. Получить счет: `invoices.get`.
2. `acts.prepare_create`

```json
{
  "invoice_id": "<invoice-id>",
  "date": "31.08.2026"
}
```

3. После подтверждения вызвать `acts.commit_create`.
4. Создать файлы:

```text
acts.render_pdf
acts.export_upd_xml
acts.validate_upd_xml
```

### Неоплаченные Счета

```text
reports.unpaid_invoices
```

или:

```text
invoices.list_unpaid
```

Важно: без банковской интеграции статус оплаты считается ручным.

### Отметить Счет Оплаченным

1. `invoices.prepare_mark_paid`
2. Подтверждение пользователя.
3. `invoices.mark_paid`

```json
{
  "confirmation_token": "<token-from-prepare>",
  "idempotency_key": "mark-paid-<invoice-id>-<date>"
}
```

## Resources

Hermes может читать MCP resources:

```text
accounting://organization/current
accounting://counterparties/{id}
accounting://contracts/{id}
accounting://invoices/{id}
accounting://acts/{id}
accounting://documents/{id}/files
accounting://reports/unpaid-invoices
```

## Ошибки

MCP tools возвращают структурированный результат:

```json
{
  "ok": false,
  "error": {
    "code": "COUNTERPARTY_NOT_FOUND",
    "message": "Контрагент не найден",
    "recoverable": true,
    "suggested_action": "Уточните ИНН или создайте контрагента"
  }
}
```

Hermes должен показывать `message` пользователю и использовать `suggested_action` для следующего шага.

Основные коды:

```text
VALIDATION_ERROR
UNAUTHORIZED
FORBIDDEN
COUNTERPARTY_NOT_FOUND
COUNTERPARTY_DUPLICATE
CONTRACT_NOT_FOUND
MULTIPLE_CONTRACTS_FOUND
DOCUMENT_NOT_FOUND
DOCUMENT_DUPLICATE
CONFIRMATION_EXPIRED
CONFIRMATION_MISMATCH
XML_VALIDATION_FAILED
STORAGE_ERROR
```

## Что Не Делать В Hermes

- Не передавать в MCP произвольные SQL-запросы.
- Не передавать произвольные пути файлов.
- Не доверять сумме, рассчитанной LLM: MCP пересчитывает строки сам.
- Не вызывать `commit_*` без явного подтверждения.
- Не считать документ отправленным в СБИС: отправка не реализована, доступен только XML export.
- Не использовать `НДС 0%`; для основной организации используется `без НДС`.

## Acceptance Smoke Test

После подключения Hermes должен уметь:

1. Получить список MCP tools.
2. Прочитать `accounting://organization/current`.
3. Выполнить `counterparties.search` по `ЦентрТТМ`.
4. Выполнить `invoices.prepare_create` и получить `confirmation_token`.
5. После подтверждения выполнить `invoices.commit_create`.
6. Выполнить `acts.prepare_create` с `invoice_id`.
7. После подтверждения выполнить `acts.commit_create`.
8. Выполнить `acts.export_upd_xml` и `acts.validate_upd_xml`.
