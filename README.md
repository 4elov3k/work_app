# work_app

Внутреннее приложение для бухгалтерии ИП и сопутствующих процессов. Состоит из Next.js фронтенда и Go бэкенда, разделённых на три фичи:

- **Документы** — счета, акты, договоры, контрагенты; интеграция с Saby (ЭДО) и `accounting-mcp` (управление документами через Hermes голосом/текстом)
- **Redmine** — дашборд проектов с контрольными срезами и отчётными датами (`redmine_project_control_events`)
- **Звонари** — синхронизация звонков с OnlinePBX, транскрипция и AI-аналитика через Hermes

## Стек

- **Frontend**: Next.js 16.3.1 (React 19.2.8), TypeScript, Tailwind CSS, Radix UI
- **Backend**: Go 1.25, Chi Router, PostgreSQL (`database/sql`, без ORM)
- **Внешние сервисы**: Hermes (OCR, транскрипция, аналитика звонков, sheets-sync), Saby (ЭДО), OnlinePBX, DaData

## Структура репозитория

```
work_app/
├── src/app/               # Next.js роутинг: [customer]/invoices, redmine, zvonari
├── src/lib/api/           # API-клиент по фичам: documents, redmine, zvonari, client
├── backend/
│   ├── cmd/server/        # Точка входа Go-сервера
│   ├── internal/          # accounting, redmine, zvonari, pbx, transcribe, docparse, ...
│   ├── migrations/        # SQL миграции (см. backend/README.md)
│   └── README.md          # Подробности бэкенда: endpoints, переменные окружения
├── docs/                  # ТЗ и описания сервисов (accounting-mcp, интеграция с Hermes)
└── docker-compose.yml     # Postgres + backend в контейнерах
```

## Запуск для разработки

### 1. База данных и бэкенд

См. [`backend/README.md`](backend/README.md) — там подробно расписаны миграции, переменные окружения и запуск Go-сервера. Коротко:

```bash
cd backend
createdb invoices_db
# применить миграции из backend/migrations/ по порядку
cp .env.example .env  # заполнить DATABASE_URL и остальные переменные
go run cmd/server/main.go
```

Либо через Docker Compose (поднимает Postgres с уже применёнными миграциями и сам бэкенд):

```bash
docker-compose up
```

### 2. Frontend

```bash
npm install
cp .env.example .env.local  # при необходимости
npm run dev
```

Приложение поднимется на `http://localhost:3000`, ожидая бэкенд на `http://localhost:8080` (см. `NEXT_PUBLIC_*` переменные, если заданы отдельно от корневого `.env.example`).

### 3. Проверки

```bash
npm run lint
npx tsc --noEmit
cd backend && go build ./... && go vet ./...
```

Тестового покрытия на фронтенде пока нет; на бэкенде частично покрыты `accounting`, `docparse`, `saby`, `export/updxml` (`go test ./...`).

## Дальнейшие планы

См. [`docs/roadmap.md`](docs/roadmap.md).
