# Carstats Server

- `raw_vehicle_stats` — временное хранилище входящих записей
- `vehicle_type_aggregates` — постоянный агрегат
- cron-задача периодически переносит данные из `raw_vehicle_stats` в агрегат
- при чтении статистики сервер учитывает и агрегированные данные, и ещё не перенесённые raw-записи

## Запуск

Требования:
- Docker
- Docker Compose
- Go 1.22+ если запускать без Docker

```bash
make up
```

После запуска будут доступны:
- gRPC: `localhost:50051`
- HTTP gateway: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/`
- OpenAPI: `http://localhost:8080/openapi.json`

## Основные HTTP ручки

Регистрация:

```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123"}'
```

Логин:

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123"}'
```

Отправка статистики:

```bash
curl -X POST http://localhost:8080/v1/stats \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"vehicle_type":"sedan","fuel_liters":20.5,"distance_km":250,"recorded_at":"2026-04-21T10:30:00Z"}'
```

Получение общей статистики:

```bash
curl http://localhost:8080/v1/stats/{vehicle_type} \
  -H "Authorization: Bearer <token>"
```

## Переменные окружения

- `DATABASE_URL` — строка подключения к Postgres
- `GRPC_ADDR` — адрес gRPC сервера, по умолчанию `:50051`
- `HTTP_ADDR` — адрес HTTP gateway, по умолчанию `:8080`
- `AUTH_TOKEN_TTL` — TTL токена, по умолчанию `24h`
- `AGGREGATION_SCHEDULE` — расписание cron, по умолчанию `@every 5m`

## Быстрые команды

```bash
make up
make test
make logs
make down
```