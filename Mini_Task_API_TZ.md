# Техническое задание: Mini Task API на Go

**Цель:** реализовать учебный REST API на стандартной библиотеке Go, закрепив `net/http`, JSON, структуры, указатели, слайсы, карты, интерфейсы, обработку ошибок, middleware, context, goroutines, channels, `select`, `sync.Mutex`, `sync.RWMutex`, `sync.WaitGroup`, `sync/atomic`, файловые операции и graceful shutdown.

## 1. Ограничения и стек

- Go, преимущественно стандартная библиотека; **без Gin, Echo, Fiber и ORM**.
- HTTP: `net/http`, JSON: `encoding/json`.
- Хранение: **in-memory**, через `map` и/или `slice` (без БД).
- Авторизация: сессии в cookie.
- Архитектура: **Handler → Service → Repository**.
- Для безопасного хеширования паролей разрешена внешняя библиотека `golang.org/x/crypto/bcrypt`.
- Не создавать избыточные абстракции ради архитектуры; интерфейсы — когда они помогают разделить зависимости.

## 2. Структура проекта

```text
mini-api/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── task/
│   │   ├── model.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   ├── auth/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── middleware.go
│   ├── user/
│   │   ├── model.go
│   │   └── repository.go
│   ├── middleware/
│   │   ├── logging.go
│   │   ├── recovery.go
│   │   └── metrics.go
│   └── server/
│       └── server.go
├── static/
├── uploads/
├── go.mod
└── go.sum
```

**Назначение:**
- `cmd/server/main.go`: сборка зависимостей, запуск, ожидание сигналов, graceful shutdown.
- `internal/server`: маршруты, общая конфигурация HTTP-сервера и middleware.
- `internal/task/handler.go`: чтение HTTP-запросов, валидация формата, HTTP-ответы.
- `internal/task/service.go`: правила работы с задачами, не зависит от HTTP.
- `internal/task/repository.go`: хранение задач, mutex, atomic ID.
- `internal/auth`: логин, логаут, сессии, auth middleware.
- `internal/user`: модель и хранение пользователей.
- `internal/middleware`: общие logging, recovery, metrics.
- `static`: общедоступные статические файлы; `uploads`: загруженные файлы.

В Go файлы внутри одной директории обычно относятся к одному пакету; разделение на файлы не означает разделение на отдельные пакеты.

## 3. Модели

```go
type User struct {
    ID           int64
    Username     string
    PasswordHash string
    IsAdmin      bool
}

type Task struct {
    ID          int64  `json:"id"`
    UserID      int64  `json:"user_id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Done        bool   `json:"done"`
}

type Session struct {
    UserID    int64
    ExpiresAt time.Time
}
```

Хранилища должны быть конкурентно безопасными. ID задач — через `atomic.Int64`. Администратора для тестирования можно создать при старте приложения.

## 4. Маршруты

| Метод | Путь | Доступ | Задача / инструменты |
|---|---|---|---|
| GET | `/health` | Public | `200`, `{"status":"ok"}`; `ResponseWriter`, `json.Encoder` |

| POST | `/register` | Public | JSON username/password; проверка уникальности, bcrypt, `201`; Decoder, map, mutex |
| POST | `/login` | Public | Проверка пароля, безопасный случайный session ID, `Set-Cookie`; `crypto/rand`, cookie |
| POST | `/logout` | Auth | Удалить сессию и инвалидировать cookie |

| GET | `/tasks` | Auth | Задачи текущего пользователя, фильтр `?done=true/false`; query, slices, RWMutex |
| GET | `/tasks/{id}` | Auth | `r.PathValue("id")`, преобразование ID, проверка владельца, `404` |
| POST | `/tasks` | Auth | Создать задачу, `201`; JSON, `atomic.Int64.Add`, mutex |
| PATCH | `/tasks/{id}` | Auth | Частично изменить `title`, `description`, `done`; DTO с указателями, mutex |
| DELETE | `/tasks/{id}` | Auth | Удалить задачу из slice, `204`; `slices.Delete`/`append`, mutex |

| GET | `/admin/stats` | Admin | JSON requests/users/tasks; metrics atomic + безопасное чтение хранилищ |

| POST | `/avatar` | Auth | Multipart upload, ограничение размера и типа, безопасное имя; FormFile, io.Copy, defer |
| GET | `/avatar/{name}` | Auth | Отдать загруженный файл через ServeFile; исключить path traversal и чужой доступ |
| GET | `/static/...` | Public | FileServer + StripPrefix |

| GET | `/external/{id}` | Auth | Получить JSON внешнего API; Client, Transport, RequestWithContext, timeout |
| POST | `/tasks/process` | Auth | Worker pool из 3 goroutine; channels, select, WaitGroup, context |

| GET | `/debug/panic` | Dev only | Временная ручка для тестирования recovery; не включать в production |

### 4.1. Форматы основных запросов

**POST /register**

```http
HEADERS: Content-Type: application/json
QUERY: нет
BODY: {"username":"dima","password":"12345678"}
```

Проверять уникальность username и длину пароля. Хранить только безопасный хеш. Не разрешать пользователю самостоятельно установить `IsAdmin`.

**POST /login**

```http
HEADERS: Content-Type: application/json
QUERY: нет
BODY: {"username":"dima","password":"12345678"}
```

Создать криптографически случайный session ID, сохранить `sessionID → Session`, отправить cookie `HttpOnly`, `SameSite`, `MaxAge` (при HTTPS также `Secure`). Неверные данные → `401`.

**GET /tasks?done=true**

```http
HEADERS: Cookie: session_id=...
QUERY: done=true
BODY: нет
```

Без query — все задачи пользователя. `done=true` — завершённые, `done=false` — незавершённые. Невалидное значение → `400`.

**POST /tasks**

```http
HEADERS: Content-Type: application/json; Cookie: session_id=...
QUERY: нет
BODY: {"title":"Learn Go","description":"Build an API"}
```

Название не должно быть пустым. `UserID` назначает сервер из контекста авторизации, а не из body. Создание → `201` и JSON задачи.

**PATCH /tasks/{id}**

```http
HEADERS: Content-Type: application/json; Cookie: session_id=...
QUERY: нет
BODY: {"done":true}
```

Отсутствующее поле не меняется. Для различения отсутствующего `done` и `false` использовать `*bool` или другой явный механизм. Проверять владельца.

**POST /tasks/process**

```http
HEADERS: Content-Type: application/json; Cookie: session_id=...
QUERY: нет
BODY: {"task_ids":[1,2,3,4,5]}
```

Обрабатывать только задачи текущего пользователя. Три worker, имитация работы 1–3 секунды на задачу. Вернуть результаты; при отмене запроса корректно остановить goroutine, не допускать зависания каналов.

**POST /avatar**

```http
HEADERS: Content-Type: multipart/form-data; Cookie: session_id=...
QUERY: нет
BODY: avatar=[image.png]
```

Использовать `http.MaxBytesReader`, `r.FormFile`, ограничение типов/размера, случайное безопасное имя файла. Файл должен быть привязан к пользователю; чужие аватары не отдавать.

**GET /external/{id}**

Пример источника: `https://jsonplaceholder.typicode.com/todos/{id}`. Использовать один переиспользуемый `http.Client` с `Transport`, timeout и `NewRequestWithContext`. Проверять HTTP-статус внешнего ответа отдельно от ошибки `Do`. Закрывать `resp.Body`. Ошибки внешнего сервиса преобразовывать в корректные ответы (`502` / `504`, если применимо).

## 5. Middleware

Реализовать:

1. **Logging:** метод, путь, статус и длительность запроса; для захвата статуса при необходимости обернуть `ResponseWriter`.
2. **Recovery:** `defer` + `recover` для panic в goroutine обработки запроса, возвращать `500` там, где ответ ещё не отправлен.
3. **Metrics:** счётчик запросов через `atomic.Int64.Add(1)`.
4. **Auth:** прочитать cookie, проверить сессию/срок, положить userID в `context.WithValue`, передать `r.WithContext(ctx)`.
5. **Admin:** после Auth проверить права, иначе `403`.

Схема:

```text
Request → Recovery → Logging → Metrics → ServeMux
                                     ├── Public
                                     ├── Auth → protected routes
                                     └── Auth → Admin → /admin/stats
```

## 6. Конкурентность

- Защитить все разделяемые map/slice через `sync.Mutex`/`sync.RWMutex`.
- Генерация ID задач — `atomic.Int64.Add(1)`.
- Счётчик HTTP-запросов — `atomic.Int64`.
- Не считать последовательность `Load` + `Store` автоматически атомарной.
- Worker pool: три worker, канал задач, канал результатов, `WaitGroup`, `select` по `ctx.Done()`.
- Не оставлять goroutine заблокированными при отмене HTTP-запроса.

## 7. HTTP-сервер и завершение

Использовать `http.Server` с самостоятельно подобранными:

```go
ReadHeaderTimeout
ReadTimeout
WriteTimeout
IdleTimeout
MaxHeaderBytes
```

При `SIGINT`/`SIGTERM` вызвать `Shutdown(ctx)` с ограниченным временем ожидания. Нормально обрабатывать `http.ErrServerClosed`. Не считать `ListenAndServeTLS` обязательным: локально достаточно HTTP; HTTPS можно настроить позже через reverse proxy.

## 8. Общие требования

- Handler занимается HTTP, service — правилами приложения, repository — данными.
- Корректные HTTP-статусы: `200`, `201`, `204`, `400`, `401`, `403`, `404`, `500`, `502`, `504` по ситуации.
- Проверять ошибки, не игнорировать `err`.
- Ограничивать размер входного body там, где это необходимо.
- Закрывать файлы и тела HTTP-ответов.
- Пароли не хранить в открытом виде; session ID создавать через `crypto/rand`.
- Проверять права доступа к задачам и загруженным файлам.
- Не допускать path traversal при выдаче файлов.
- Добавить тесты на ключевую бизнес-логику и HTTP handlers.
- Запустить `go vet ./...`, `go test ./...`, `go test -race ./...`.

## 9. Этапы реализации

- [ ] **Этап 1.** Инициализация модуля, структура проекта, `GET /health`, `ServeMux`.
- [ ] **Этап 2.** CRUD задач: модели, handler/service/repository.
- [ ] **Этап 3.** `Mutex`/`RWMutex`, atomic ID, проверка гонок.
- [ ] **Этап 4.** Регистрация, логин, logout, сессии, cookie, auth context.
- [ ] **Этап 5.** Logging, recovery, metrics, admin middleware.
- [ ] **Этап 6.** Загрузка и выдача файлов, статическая директория.
- [ ] **Этап 7.** Исходящий HTTP-клиент, Transport, context и timeout.
- [ ] **Этап 8.** Worker pool, channels, select, WaitGroup, отмена.
- [ ] **Этап 9.** Graceful shutdown, тесты и race detector.

## 10. Критерии готовности

1. `go run ./cmd/server` запускает сервер.
2. Пользователь регистрируется, входит и выходит.
3. Пользователь создаёт, получает, фильтрует, меняет и удаляет **только свои** задачи.
4. Администратор получает статистику, обычный пользователь — `403`.
5. Загрузка/выдача файлов работают безопасно.
6. `/external/{id}` обращается к внешнему API с timeout и context.
7. `/tasks/process` выполняет конкурентную обработку с ограничением в три worker.
8. Отмена запроса останавливает связанную обработку.
9. Ctrl+C приводит к корректному завершению сервера.
10. Проверки и тесты проходят без обнаруженных гонок.

---

**Принцип проекта:** сначала работающий и понятный код, затем улучшение архитектуры. Не плодить интерфейсы и папки без необходимости. Главная цель — осознанно использовать механизмы Go, а не механически копировать готовое решение.
