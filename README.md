# Carstats Server

- `user_vehicle_stats` — временное хранилище актуальной статистики: одна запись на пользователя
- повторный `POST /v1/stats` от того же пользователя перезаписывает его запись
- агрегаты для чтения считаются напрямую по актуальным пользовательским записям

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
  -d '{"vehicle_id":"uuid-123","brand":"Kia","model":"Rio","vehicle_type":"sedan","manual_avg_consumption_l_per_100km":8.2,"obd_avg_consumption_l_per_100km":8.6,"avg_refuel_liters":34.1,"avg_distance_between_refuels_km":410,"manual_records_count":5,"telemetry_samples_count":1861,"updated_at":"2026-04-24T19:00:00Z"}'
```

Получение общей статистики по модели:

```bash
curl "http://localhost:8080/v1/stats/model?brand=Kia&model=Rio" \
  -H "Authorization: Bearer <token>"
```

Получение общей статистики по типу:

```bash
curl "http://localhost:8080/v1/stats/type?vehicle_type=sedan" \
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
