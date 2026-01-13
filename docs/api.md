# API документация (backend)

Базовый URL: `http://localhost:8080`

## Общие правила
- Все суммы — целые числа в копейках (`*_cents`, `int64`).
- Даты периода: `YYYY-MM-DD`.
- Время: RFC3339 (`created_at`, `updated_at`).
- UUID — стандартные UUIDv4 в строке.
- Все JSON запросы: `Content-Type: application/json`.
- Авторизация: `Authorization: Bearer <access_token>` для всех `/api/v1/*`, кроме `/auth/*` и `/health`.

## Ошибки
Чаще всего:
- `{"error":"..."}` — ошибки валидации/логики из хендлеров.
- `{"message":"..."}` — ошибки middleware (например, неверный токен).

## Rate limiting
- `/api/v1/auth/*` и `/api/v1/ai/*` могут возвращать `429 Too Many Requests`.

## Health
`GET /health`

Ответ:
```json
{"status":"ok"}
```

## Авторизация
### Регистрация
`POST /api/v1/auth/register`
```json
{"email":"user@example.com","password":"Pass1234","name":"Иван"}
```
Ответ:
```json
{
  "access_token":"...",
  "refresh_token":"...",
  "user":{"id":"...","email":"user@example.com","name":"Иван"}
}
```

### Вход
`POST /api/v1/auth/login`
```json
{"email":"user@example.com","password":"Pass1234"}
```
Ответ: как при регистрации.

### Обновление токена
`POST /api/v1/auth/refresh`
```json
{"refresh_token":"..."}
```
Ответ: как при регистрации.

### Выход
`POST /api/v1/auth/logout`
```json
{"refresh_token":"..."}
```
Ответ: `204 No Content`.

### Текущий пользователь
`GET /api/v1/auth/me`
Ответ:
```json
{"user":{"id":"...","email":"user@example.com","name":"Иван"}}
```

## Планы бюджета
### Список планов
`GET /api/v1/plans`
Ответ:
```json
{"plans":[{"id":"...","title":"...","budget_cents":0,"period_start":"YYYY-MM-DD","period_end":"YYYY-MM-DD","background_color":"#RRGGBB","is_ai_generated":true,"spent_cents":0,"remaining_cents":0,"created_at":"...","updated_at":"..."}]}
```
`spent_cents` считается только по `is_completed=true`.

### Архив
`GET /api/v1/plans/archive`
Ответ: формат как у списка.

### Создать план
`POST /api/v1/plans`
```json
{
  "title":"Ноябрь",
  "budget_cents":500000,
  "period_start":"2024-11-01",
  "period_end":"2024-11-30",
  "background_color":"#FDF7F7",
  "is_ai_generated":false
}
```
Ответ: `PlanResponse`.

### Получить план
`GET /api/v1/plans/{id}`
Ответ:
```json
{
  "plan":{...PlanResponse...},
  "categories":[
    {"id":"...","title":"...","category_type":"mandatory","sort_order":0,"items":[...]}
  ],
  "notes":[{"id":"...","content":"...","note_type":"ai","sort_order":0,"created_at":"...","updated_at":"..."}]
}
```

### Обновить план
`PUT /api/v1/plans/{id}`
Payload такой же, как при создании (все поля обязательны).
Ответ: `PlanResponse`.

### Удалить план
`DELETE /api/v1/plans/{id}` → `204 No Content`.

### Перестановка категорий
`PATCH /api/v1/plans/{id}/reorder`
```json
{"category_ids":["uuid1","uuid2", "..."]}
```
Важно: нужно передать **все** категории плана в новом порядке.

### Дублировать план
`POST /api/v1/plans/{id}/duplicate`
Ответ: `PlanResponse` (копия плана).

### Экспорт JSON
`GET /api/v1/plans/{id}/export/json`
Возвращает JSON файл с `PlanDetailResponse`.

### Экспорт CSV
`GET /api/v1/plans/{id}/export/csv?type=items|notes`
`type` по умолчанию `items`.

## Категории и расходы
### Создать расход
`POST /api/v1/plans/{planId}/categories/{categoryId}/items`
```json
{"title":"Аренда","amount_cents":200000,"priority_color":"red","is_completed":false}
```
Ответ:
```json
{"id":"...","title":"...","amount_cents":0,"priority_color":"red","is_completed":false,"sort_order":0}
```

### Обновить расход
`PUT /api/v1/items/{itemId}`
```json
{"title":"Аренда","amount_cents":220000,"priority_color":"red"}
```

### Удалить расход
`DELETE /api/v1/items/{itemId}` → `204 No Content`.

### Переключить выполненность
`PATCH /api/v1/items/{itemId}/toggle`
```json
{"is_completed":true}
```
Если тело пустое — статус переключается на противоположный.

### Изменить порядок расходов
`PATCH /api/v1/items/{itemId}/reorder`
```json
{"item_ids":["uuid1","uuid2", "..."]}
```
Важно: передать **все** расходы категории в новом порядке.

### Изменить цвет приоритета
`PATCH /api/v1/items/{itemId}/color`
```json
{"priority_color":"yellow"}
```

## Заметки
### Список заметок плана
`GET /api/v1/plans/{planId}/notes`
Ответ:
```json
{"notes":[{"id":"...","content":"...","note_type":"user","sort_order":0,"created_at":"...","updated_at":"..."}]}
```

### Создать заметку
`POST /api/v1/plans/{planId}/notes`
```json
{"content":"...", "note_type":"user"}
```

### Обновить заметку
`PUT /api/v1/notes/{noteId}`
```json
{"content":"...", "note_type":"user"}
```

### Удалить заметку
`DELETE /api/v1/notes/{noteId}` → `204 No Content`.

### Перестановка заметок
`PATCH /api/v1/notes/{noteId}/reorder`
```json
{"note_ids":["uuid1","uuid2", "..."]}
```
Важно: передать **все** заметки плана в новом порядке.

## AI
### Генерация плана
`POST /api/v1/ai/generate-plan`
```json
{
  "period_start":"2024-11-01",
  "period_end":"2024-11-30",
  "budget_cents":5000000,
  "currency":"RUB",
  "user_data":{
    "period":"Ноябрь 2024",
    "income":[{"source":"ЗП","amount_cents":7000000}],
    "mandatory_expenses":[{"title":"Аренда","amount_cents":3000000}],
    "optional_expenses":[{"title":"Спорт","amount_cents":200000}],
    "assets":[],
    "debts":[],
    "additional_notes":"Сделай реалистично"
  }
}
```
Ответ: `201` + `PlanDetailResponse`.  
При ошибке AI создается шаблонный план (`is_ai_generated=false`) с заметкой.

### Анализ расходов
`POST /api/v1/ai/analyze-spending`
```json
{"plan_id":"uuid","currency":"RUB"}
```
Ответ:
```json
{"advices":[{"id":"...","content":"...","note_type":"ai","sort_order":0,"created_at":"...","updated_at":"..."}]}
```
AI‑заметки перезаписываются (старые удаляются).

### Получить AI‑советы
`GET /api/v1/ai/advices/{planId}`
Ответ: `{"advices":[...NoteResponse...]}`.

## Статистика
### Обзор
`GET /api/v1/stats/overview`
```json
{"total_plans":0,"active_plans":0,"archived_plans":0,"total_budget_cents":0,"total_spent_cents":0,"remaining_cents":0}
```

### Траты по категориям
`GET /api/v1/stats/spending-by-category?plan_id=uuid`
```json
{"plan_id":"uuid","categories":[{"category_id":"...","title":"...","category_type":"mandatory","spent_cents":0}]}
```

### Сравнение по месяцам
`GET /api/v1/stats/monthly-comparison?months=6` (1–24)
```json
{"months":[{"month":"2024-11","budget_cents":0,"spent_cents":0}]}
```

## SSE уведомления
`GET /api/v1/notifications/stream`

Формат события:
```
event: budget_updated
data: {"type":"budget_updated","timestamp":"2026-01-12T13:59:44Z","data":{"plan_id":"...","spent_cents":0,"remaining_cents":0}}
```

Типы событий:
- `connected` — при подключении.
- `budget_updated` — при создании/изменении плана/расходов.
- `ai_advices` — после генерации советов.

Примечание: требуется авторизация. В браузере `EventSource` не умеет заголовки — нужен прокси, cookie‑auth или fetch‑stream.

## Админка
Доступ ограничен `ADMIN_EMAILS`.

### Пользователи
`GET /api/v1/admin/users?limit=50&offset=0`
Ответ:
```json
{"total":0,"users":[{"id":"...","email":"...","name":"...","created_at":"...","updated_at":"..."}]}
```

### AI‑запросы
`GET /api/v1/admin/ai-requests?user_id=uuid&success=true&request_type=generate_plan&include_payloads=false&limit=50&offset=0`
Ответ:
```json
{"total":0,"requests":[{"id":"...","user_id":"...","request_type":"...","provider":"...","model":"...","success":true,"created_at":"..."}]}
```
При `include_payloads=true` добавляются `prompt`, `request_payload`, `response_payload`, `raw_response`.

### Статистика
`GET /api/v1/admin/usage?days=7`
Ответ:
```json
{"users":0,"plans":0,"ai_requests":0,"ai_success":0,"ai_fail":0,"ai_requests_by_day":[{"date":"2024-11-01","count":0}]}
```
