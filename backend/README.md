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
```

### 2. Настройка переменных окружения

Создайте файл `.env` в корне backend директории:

```bash
DATABASE_URL=postgres://username:password@localhost:5432/invoices_db
PORT=8080
CORS_ORIGIN=http://localhost:3000
```

### 3. Установка зависимостей

```bash
go mod download
```

### 4. Запуск сервера

```bash
go run cmd/server/main.go
```

Сервер запустится на http://localhost:8080

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
