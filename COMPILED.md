# Сводный контекст по проекту (актуальный единый источник)

## 0) Краткое ТЗ (поверхностный уровень)
- Нужно приложение для хранения контрагентов компании.
- В процессе работы формируются документы: счета и акты выполненных работ.
- Реализуются услуги с ценой, которые включаются в документы.
- Приложение должно помогать генерировать документы, сохранять как PDF и печатать.
- Часть логики уже реализована, но требуется структурировать и в будущем задеплоить приложение.

## 1) Итоговый статус и контекст
- Проект: система управления счетами и актами контрагентов, современный web‑стек. Источник: `README.md`.
- Заявлен статус: «Production Ready», результат миграции завершен. Источники: `FINAL_RESULT.md`, `MIGRATION_COMPLETE.md`, `README.md`.
- Ключевой итог: миграция с PocketBase на Go + PostgreSQL и обновление UI на shadcn/ui. Источники: `MIGRATION_COMPLETE.md`, `SHADCN_UI_MIGRATION.md`, `FINAL_RESULT.md`.

## 2) Архитектура (актуальная + наследие)

### Актуальная архитектура (Go + PostgreSQL + Next.js)
- Backend: Go (Chi router), PostgreSQL, `database/sql`, CORS. Источники: `MIGRATION_COMPLETE.md`, `README.md`.
- Frontend: Next.js 15, TypeScript, Tailwind, shadcn/ui, Lucide. Источники: `README.md`, `SHADCN_UI_MIGRATION.md`.
- Централизованный API клиент: `src/lib/api.ts`, типы и единая конфигурация API_BASE. Источники: `MIGRATION_COMPLETE.md`, `FRONTEND_TEST_RESULTS.md`.
- Базовая схема данных (актуальная): `customers`, `contracts`, `invoices`, `invoice_lines`, `acts`, `act_lines`, `act_invoices`, `services`.
- API эндпоинты (актуальные):
  - `GET /api/customers`, `GET /api/customers?search=...`, `GET /api/customers/{id}`
  - `GET /api/invoices?customer_id=...`, `GET /api/invoices/{id}`, `GET /api/invoices/{id}/services`, `POST /api/invoices`
  - `POST /api/services`
  Источники: `README.md`, `MIGRATION_COMPLETE.md`.

### Наследие (PocketBase API) — исторический контекст
- `BACKEND_DOCUMENTATION.md` описывает PocketBase API (`/api/collections/...`) и внутренние рекомендации (ENV для API_URL, API клиент, типы). Это полезно как исторический контекст, но не отражает текущую реализацию после миграции. Источник: `BACKEND_DOCUMENTATION.md`.

## 3) Бизнес‑функции и сценарии

### Счета и акты
- Счета и акты разделены: `invoices` и `acts` — отдельные таблицы, акт больше не хранится в `invoices`.
- Счёт/акт всегда принадлежат договору (`contract_id`).
- Строки документов — в `invoice_lines` / `act_lines` со snapshot полей.
- Связь акт–счёт: `act_invoices` (N:M) с триггером на совпадение `contract_id`.

### Создание контрагента
- Новый endpoint `POST /api/customers` с валидацией. Источник: `CREATE_CUSTOMER_FEATURE.md`.
- UI: модальный `CreateCustomerDialog`, обязательные поля, UX‑подсказки. Источник: `CREATE_CUSTOMER_FEATURE.md`.

### PDF‑скачивание
- Кнопка «Скачать PDF» на странице документа, клиентская генерация PDF (html2canvas + jspdf), добавлена печать и подпись. Источник: `PDF_DOWNLOAD.md`.

## 4) Frontend UI/UX и шаблоны
- Полная миграция на shadcn/ui, обновлены основные страницы и таблицы, добавлены Dialog/Tabs/Table/Card. Источник: `SHADCN_UI_MIGRATION.md`.
- Стилизация печати вынесена в `form.css` (print‑only). Источник: `SHADCN_UI_MIGRATION.md`.
- Адаптивность и визуальные улучшения: сетки карточек, ховеры, пустые состояния, иконки. Источник: `SHADCN_UI_MIGRATION.md`.

## 5) Операции: запуск, перезапуск, конфигурация
- Быстрый старт (Docker + .env.local + Next.js). Источник: `README.md`.
- Полная инструкция Setup (PostgreSQL, миграции, переменные окружения, запуск backend/frontend). Источник: `SETUP.md`.
- Быстрый запуск и проверка API + базовые команды управления сервисами. Источник: `QUICK_START.md`.
- Перезапуск фронтенда с очисткой кэша Next.js и браузера. Источник: `RESTART_FRONTEND.md`.
- Отдельные шаги запуска фронтенда и диагностики ошибок. Источник: `START_FRONTEND.md`.
- Добавлена настройка `outputFileTracingRoot` в `next.config.ts` для устранения предупреждения о нескольких lockfile.

## 6) Тестирование и проверка
- Полный сценарий тестирования API + UI + граничные случаи + performance. Источник: `TESTING.md`.
- Результаты тестирования фронтенда с новым API (метрики сборки, список обновленных модулей). Источник: `FRONTEND_TEST_RESULTS.md`.
- Чек‑лист применения изменений по счетам/актам, проверки в БД и troubleshooting. Источник: `APPLY_CHANGES.md`.

## 7) Исправления и инциденты
- Исправление 404 на `GET /api/invoices/{id}/services`:
  - порядок роутов Chi
  - корректное формирование IN‑запроса для UUID
  - добавление логирования
  Источник: `FIX_SERVICES_ENDPOINT.md`.
- Зафиксированная проблема с Docker‑кэшем, порядком роутов и UUID массивами; описаны команды перепаковки. Источник: `PROBLEM_SOLVED.md`.

## 8) Ключевые артефакты/файлы (по функциям)
- Backend: `backend/cmd/server/main.go`, `backend/internal/handlers`, `backend/internal/database`, миграции в `backend/migrations`. Источник: `MIGRATION_COMPLETE.md`.
- Frontend API клиент: `src/lib/api.ts`. Источник: `MIGRATION_COMPLETE.md`, `FRONTEND_TEST_RESULTS.md`.
- Печать/шаблоны: `src/app/components/templates/InvoiceTemplate.tsx`, `CertificateTemplate.tsx`, `StampAndSignature.tsx`. Источники: `INVOICE_CERTIFICATE_DOCS.md`, `PDF_DOWNLOAD.md`.
- Дублирование документа: `src/app/[customer]/[invoice]/components/duplicate.tsx`. Источник: `INVOICE_CERTIFICATE_DOCS.md`.
- Создание контрагента: `src/app/components/CreateCustomerDialog.tsx`. Источник: `CREATE_CUSTOMER_FEATURE.md`.

## 9) Рекомендации и next steps (для оптимизации/архитектуры)
- Обновление/централизация конфигурации API и типизации уже реализованы в новой архитектуре (Go API + `src/lib/api.ts`), но важно не путать с legacy PocketBase из `BACKEND_DOCUMENTATION.md`. Источники: `MIGRATION_COMPLETE.md`, `FRONTEND_TEST_RESULTS.md`, `BACKEND_DOCUMENTATION.md`.
- Следующие шаги (опционально): краткосрочные/среднесрочные/долгосрочные улучшения указаны в миграционных документах. Источники: `MIGRATION_COMPLETE.md`, `SHADCN_UI_MIGRATION.md`.

---

## 10) Аудит проблем (текущий код) — первичный список

### Критичные (блокируют/ломают)
- Несоответствие схемы БД и кода по `contract_number`.
  - Миграция `005_add_contract_to_customers.sql` удаляет `contract_number` из `invoices`.
  - Код backend продолжает читать/писать `invoices.contract_number` (`backend/internal/database/invoices.go`, `backend/internal/models/invoice.go`).
  - Frontend отображает `invoice.contract_number` (`src/app/invoices/page.tsx`, `CertificateTemplate`).
  - Итог: при применении 005 запросы к invoices будут падать с ошибкой отсутствия колонки.
  - Принято решение: источник истины — `invoices.contract_number`. Добавлена миграция `006_repair_contract_number.sql`, которая восстанавливает колонку и при необходимости переносит значения из `customers.contract_number`.

### Высокий приоритет
- Нет транзакционного создания документа + услуг с точки зрения API.
  - UI сначала создает услугу, затем документ. Если второй шаг падает, остается «висячая» услуга.
  - Решение: расширен `POST /api/invoices` — теперь принимает `services` (массив `{name, price}`) и создает услуги и связи в одной транзакции.
- Дублирование документа не проверяет уникальность номера.
  - Исправлено: добавлена проверка уникальности в `DuplicateInvoice`, при конфликте возвращается 400.
- Серверные компоненты используют клиентский API‑клиент без управления кешированием.
  - `src/app/[customer]/[invoice]/page.tsx` и `src/app/components/components/form.tsx` используют `src/lib/api.ts` с `fetch`, что на сервере кэшируется по умолчанию и может показывать устаревшие данные.
  - Исправлено: добавлен `src/lib/api.server.ts` с `cache: \"no-store\"` и `revalidate: 0`, server components переведены на него.

### Средний приоритет
- Дублирование UI‑логики списков счетов/актов.
  - `src/app/[customer]/components/invoicesList.tsx` и `certificateList.tsx` почти идентичны.
  - Исправлено: заменено на общий `DocumentList` с параметром `documentType`.
- Формат даты хранится строкой `dd.mm.yyyy`.
  - Усложняет сортировку/валидность; лучше хранить `DATE` в БД и форматировать в UI.
- Валидация данных на backend неполная.
  - `CreateCustomer` не валидирует `address`, формат/длину `inn`.
  - `CreateInvoice` не валидирует `document_type` (enum), `date` (формат).
  - Исправлено: добавлена валидация `address`, длины `inn`, `document_type` и формата даты `dd.mm.yyyy`.

### Низкий приоритет
- DEBUG‑логи в `GetInvoiceWithServices` всегда включены.
  - Исправлено: логи активируются только при `LOG_LEVEL=debug`.
- Поиск контрагентов в `src/app/page.tsx` без debounce.
  - Исправлено: добавлен debounce 300мс.

---

## 11) Последние изменения (февраль 2026)
- ESLint обновлен до v9 с flat-config (`eslint.config.mjs`), `npm run lint` проходит без ошибок.
- Приведены `catch` к `unknown` и корректной обработке ошибок в UI-формах (редактирование документов, список услуг).
- `tailwind.config.ts` переведен на ESM-импорт для `tailwindcss-animate` (без `require`).
- Исправлен `handleResponse`/обработка ошибок в API-клиенте и унифицировано поведение UI при сетевых ошибках.
- Добавлены PATCH-методы и фильтр `archived` для счетов/актов, логика «архивности» и запрет редактирования архивных документов.
- Автонумерация: договоры стартуют с 700, счета/акты — с 3000, есть ручной ввод и эндпоинты `next-number`.
- UI: вкладки для счетов/актов на странице договора, карточки кликабельны целиком, исправлены пересечения кнопок/бейджей.
- Исправлена инициализация `loadNextNumber` в списке документов (tsc без ошибок).

## 11) Статус сборки (последняя проверка)
- Сборка прошла успешно после отключения Google Fonts и перехода на локальный шрифт (см. ниже).
- 2026-02-25: backend контейнер пересобран `docker compose up -d --build backend`.

## 12) Линт и шрифты (последние изменения)
- Подключен локальный шрифт IBM Plex Sans через `@fontsource/ibm-plex-sans`:
  - Импорты веса 400/500/600 в `src/app/layout.tsx`.
  - `fontFamily.sans` расширен в `tailwind.config.ts`.
- Добавлен `.eslintrc.json` с `next/core-web-vitals` и `next/typescript`.
- `npm run lint` требует установленный `eslint`, установка через npm была невозможна из‑за сетевой ошибки `EAI_AGAIN` к `registry.npmjs.org`.

## 14) API-клиент (последние исправления)
- `src/lib/api.ts`: `handleResponse` теперь корректно обрабатывает не-JSON ответы, пустые тела и статус `204`, а также формирует более информативное сообщение об ошибке.

## 15) Миграции БД (последние исправления)
- Применены миграции: `003_add_document_type.sql`, `004_add_contract_number.sql`, `005_add_contract_to_customers.sql`, `006_repair_contract_number.sql`.
- Это устраняет ошибку `column "document_type" does not exist` при запросах к `/api/invoices`.
- 2026-02-25: применена `007_contracts_acts_lines.sql` (contracts/acts/lines + ограничения и триггеры).
- 2026-02-25: применена `008_contract_topics.sql` (добавлены `topic`, расширен `status`, ограничения по тематикам).
- 2026-02-25: применена `009_archive_flags.sql` (флаг `archived` для счетов/актов + индексы).

## 16) Новая доменная модель (Contract/Invoice/Act/Lines)
- Введены сущности: `contracts`, `acts`, `invoice_lines`, `act_lines`, `act_invoices` и строгие связи/ограничения.
- `invoices` больше не содержит `document_type`; акты вынесены в отдельную таблицу `acts`.
- Счёт и акт всегда принадлежат договору (`contract_id`), номера уникальны в рамках договора.
- Строки документов содержат snapshot (title/price/unit/vat) + опциональный `service_id` (ON DELETE SET NULL).
- Связь акт–счёт через `act_invoices` с триггером, запрещающим связывать документы разных договоров.

## 17) Новые миграции
- `backend/migrations/007_contracts_acts_lines.sql`:
  - Создаёт `contracts`, `acts`, `invoice_lines`, `act_lines`, `act_invoices`.
  - Добавляет `contract_id`, `status`, `total_amount` в `invoices`.
  - Мигрирует данные из `invoice_services`, переносит акты из `document_type=certificate`.
  - Удаляет `invoice_services` и `document_type`.
  - Добавляет FK/UNIQUE/CHECK/индексы и триггеры целостности.

## 18) Новые/обновлённые API эндпоинты
- Contracts:
  - `GET /api/contracts?customer_id=...`
  - `GET /api/contracts/{id}`
  - `POST /api/contracts`
  - `DELETE /api/contracts/{id}`
- Invoices:
  - `GET /api/invoices?customer_id=...&contract_id=...`
  - `GET /api/invoices/{id}`
  - `GET /api/invoices/{id}/services`
  - `POST /api/invoices`
  - `POST /api/invoices/duplicate`
  - `POST /api/invoices/{id}/lines` (добавить строку)
  - `POST /api/invoices/{id}/act` (создать акт из счета)
  - `PATCH /api/invoices/{id}` (редактирование + архив)
  - `DELETE /api/invoices/{id}`
- Acts:
  - `GET /api/acts?customer_id=...&contract_id=...`
  - `GET /api/acts/{id}`
  - `GET /api/acts/{id}/services`
  - `POST /api/acts`
  - `POST /api/acts/{id}/invoices`
  - `POST /api/acts/{id}/lines` (добавить строку)
  - `PATCH /api/acts/{id}` (редактирование + архив)
  - `DELETE /api/acts/{id}`
 - Services:
  - `GET /api/services`
  - `POST /api/services`
  - `DELETE /api/services/{id}`

## 19) Валидации и транзакции
- Создание счёта/акта — только при существующем договоре.
- Строки документов создаются транзакционно.
- Уникальность номера счёта/акта в рамках договора.
- Договоры: обязательны `topic` и валидный `status` (active/archived + legacy), ограничение тематики в БД.
- Номера договоров/счетов/актов валидируются как числовые.
- Архивные акты/счета запрещено изменять (кроме вывода из архива).

## 20) Минимальные тесты целостности
- `backend/tests/constraints.sql`:
  - нельзя создать invoice без contract
  - нельзя связать act с invoice другого договора
  - удаление service не ломает историю (service_id -> NULL)
  - уникальность номера в рамках договора
- 2026-02-25: тесты обновлены (первый кейс ловит ошибки триггера) и успешно выполнены в транзакции.

## 21) UI для договоров/услуг и удалений
- Внутри карточки контрагента добавлены вкладки: Договоры и Услуги (акты/счета открываются внутри договора).
- Добавлены формы создания и модалки подтверждения удаления для договоров, счетов, актов и услуг.
- Договоры: поля номер, дата, статус (архив/не архив), тематика (фиксированный список).
- Из карточки договора ведет переход на страницу договора со списком актов и счетов по договору.
- На страницах счета/акта доступно добавление услуги в документ.
- На странице счета доступна кнопка «Сформировать акт» на основании счета.
- Автонумерация: договоры стартуют с 700, счета/акты с 3000. В форме есть переключатель «ввести вручную».
- При конфликте номера (UNIQUE) сервер возвращает `409`, UI показывает понятное сообщение.
- Добавлено редактирование счетов/актов и флаг «В архиве», фильтр архивных в списках.

## 13) npm audit (последний отчет)
- Пользователь выполнил полный апдейт зависимостей. `next` обновлен до `^15.5.12` (ранее 15.1.3).
- Предыдущий аудит показывал уязвимости в `axios`, `brace-expansion`, `form-data`, `glob`, `jspdf`, `minimatch`, `next`.
- Требуется повторный `npm audit` для подтверждения, но он зависит от доступа к `registry.npmjs.org`.

## Список исходных документов (все .md включены в контекст)
- `APPLY_CHANGES.md`
- `BACKEND_DOCUMENTATION.md` (legacy PocketBase)
- `CREATE_CUSTOMER_FEATURE.md`
- `FINAL_RESULT.md`
- `FIX_SERVICES_ENDPOINT.md`
- `FRONTEND_TEST_RESULTS.md`
- `INVOICE_CERTIFICATE_DOCS.md`
- `MIGRATION_COMPLETE.md`
- `PDF_DOWNLOAD.md`
- `PROBLEM_SOLVED.md`
- `QUICK_START.md`
- `README.md`
- `RESTART_FRONTEND.md`
- `SETUP.md`
- `SHADCN_UI_MIGRATION.md`
- `START_FRONTEND.md`
- `TESTING.md`
