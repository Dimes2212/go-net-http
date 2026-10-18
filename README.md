# Go `net/http`: задачи, cookie-сессии и работа с файлами

Учебный REST API на стандартной библиотеке Go. Проект показывает базовую работу с `net/http`, маршрутизацией через `http.ServeMux`, middleware, контекстом запроса, cookie-сессиями, JSON-хранилищем и загрузкой файлов через `multipart/form-data`.

Ветка с текущей реализацией: `net-http-basics-and-file-handling`.

## Что реализовано

- health check;
- регистрация и вход пользователя;
- хеширование паролей через bcrypt;
- cookie-сессии со сроком жизни 24 часа;
- middleware авторизации;
- выход из аккаунта;
- создание, получение, фильтрация и частичное обновление задач;
- разграничение доступа к задачам по владельцу;
- загрузка одного вложения для задачи;
- ограничение размера вложения до 10 МБ;
- определение MIME-типа по содержимому файла;
- скачивание вложения через `http.ServeContent`;
- хранение пользователей и задач в JSON-файлах;
- защита JSON-хранилищ через `sync.RWMutex`;
- защита загрузки вложений через `sync.Mutex`.

## Стек

- Go 1.26.5;
- стандартная библиотека `net/http`;
- `http.ServeMux` с method-based patterns;
- `golang.org/x/crypto/bcrypt`;
- JSON-файлы вместо базы данных;
- локальная файловая система для вложений.

## Архитектура

Запрос проходит через следующие слои:

```text
HTTP request
    ↓
ServeMux
    ↓
Auth middleware
    ↓
Handler
    ↓
Service
    ↓
Repository / FileStorage
```

- `Handler` разбирает HTTP-запрос и формирует HTTP-ответ.
- `Service` содержит правила приложения и связывает разные хранилища.
- `Repository` читает и записывает JSON.
- `FileStorage` сохраняет и открывает файлы на диске.
- `RequireAuth` проверяет cookie и передаёт `userID` через `context.Context`.

## Структура проекта

```text
.
├── cmd/server/main.go          # сборка зависимостей и запуск сервера
├── data/
│   ├── users.json              # пользователи
│   ├── tasks.json              # задачи и метаданные вложений
│   └── attachments/            # загруженные файлы, не попадают в Git
├── internal/
│   ├── auth/
│   │   ├── handler.go          # register, login, logout
│   │   ├── middleware.go       # проверка session_id
│   │   └── service.go          # сессии и аутентификация
│   ├── server/
│   │   ├── health.go
│   │   └── routes.go
│   ├── task/
│   │   ├── file_storage.go     # сохранение и открытие файлов
│   │   ├── handler.go
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   └── user/
│       └── repository.go
├── go.mod
└── Mini_Task_API_TZ.md
```

## Запуск

Установить зависимости:

```bash
go mod download
```

Запустить проверки:

```bash
go test ./...
```

```bash
go vet ./...
```

Запустить сервер из корня проекта:

```bash
go run ./cmd/server
```

Сервер будет доступен по адресу:

```text
http://localhost:8080
```

## Маршруты

| Метод | Путь | Авторизация | Назначение |
|---|---|---:|---|
| `GET` | `/health` | нет | Проверка состояния сервера |
| `POST` | `/register` | нет | Регистрация пользователя |
| `POST` | `/login` | нет | Вход и получение cookie |
| `POST` | `/logout` | да | Завершение сессии |
| `POST` | `/tasks` | да | Создание задачи |
| `GET` | `/tasks` | да | Список задач пользователя |
| `GET` | `/tasks?done=true` | да | Выполненные задачи |
| `GET` | `/tasks?done=false` | да | Невыполненные задачи |
| `GET` | `/tasks/{id}` | да | Получение задачи |
| `PATCH` | `/tasks/{id}` | да | Частичное обновление задачи |
| `POST` | `/tasks/{id}/attachment` | да | Загрузка вложения |
| `GET` | `/tasks/{id}/attachment` | да | Скачивание вложения |

## Полный сценарий проверки

Сервер должен быть запущен в отдельном терминале:

```bash
go run ./cmd/server
```

Все следующие команды выполняются во втором терминале из корня проекта.

### 1. Подготовить переменные

Уникальное имя позволяет повторять сценарий без конфликта существующего пользователя:

```bash
API_URL='http://localhost:8080'
```

```bash
USERNAME="attachment-user-$(date +%s)"
```

```bash
PASSWORD='12345678'
```

```bash
COOKIE_JAR='/tmp/go-net-http-cookies.txt'
```

```bash
FILE_PATH='./Mini_Task_API_TZ.md'
```

### 2. Проверить сервер

```bash
curl -i "$API_URL/health"
```

Ожидаемый ответ:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{"status":"ok"}
```

### 3. Зарегистрировать пользователя

```bash
curl -i -H 'Content-Type: application/json' -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" "$API_URL/register"
```

Ожидаемый статус: `201 Created`.

Пароль должен содержать не менее восьми символов. В `users.json` сохраняется bcrypt-хеш, а не исходный пароль.

### 4. Войти и сохранить cookie

```bash
curl -i -c "$COOKIE_JAR" -H 'Content-Type: application/json' -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" "$API_URL/login"
```

Ответ содержит заголовок примерно такого вида:

```http
Set-Cookie: session_id=...; Path=/; Max-Age=86399; HttpOnly; SameSite=Lax
```

Посмотреть cookie-файл:

```bash
cat "$COOKIE_JAR"
```

Во всех защищённых запросах cookie передаётся через `curl -b`.

### 5. Создать задачу

```bash
TASK_RESPONSE=$(curl -sS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d '{"title":"Задача с вложением","description":"Проверяем загрузку и скачивание файла"}' "$API_URL/tasks")
```

Посмотреть ответ:

```bash
printf '%s\n' "$TASK_RESPONSE"
```

Получить ID задачи из JSON:

```bash
TASK_ID=$(printf '%s' "$TASK_RESPONSE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
```

```bash
printf 'TASK_ID=%s\n' "$TASK_ID"
```

### 6. Получить список задач

```bash
curl -i -b "$COOKIE_JAR" "$API_URL/tasks"
```

Только невыполненные:

```bash
curl -i -b "$COOKIE_JAR" "$API_URL/tasks?done=false"
```

Только выполненные:

```bash
curl -i -b "$COOKIE_JAR" "$API_URL/tasks?done=true"
```

### 7. Получить одну задачу

```bash
curl -i -b "$COOKIE_JAR" "$API_URL/tasks/$TASK_ID"
```

### 8. Частично обновить задачу

Можно передавать любое сочетание полей `title`, `description` и `done`:

```bash
curl -i -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' -d '{"done":true}' "$API_URL/tasks/$TASK_ID"
```

Например, обновить сразу заголовок и описание:

```bash
curl -i -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' -d '{"title":"Обновлённая задача","description":"Новое описание"}' "$API_URL/tasks/$TASK_ID"
```

### 9. Загрузить вложение

```bash
curl -i -b "$COOKIE_JAR" -F "file=@$FILE_PATH" "$API_URL/tasks/$TASK_ID/attachment"
```

Ожидаемый статус: `201 Created`.

Пример ответа:

```json
{
  "name": "Mini_Task_API_TZ.md",
  "content_type": "text/plain; charset=utf-8",
  "size": 15001
}
```

Повторная загрузка для той же задачи вернёт `409 Conflict`, потому что сейчас поддерживается только одно вложение.

### 10. Проверить метаданные вложения

```bash
curl -i -b "$COOKIE_JAR" "$API_URL/tasks/$TASK_ID"
```

После загрузки задача содержит поле:

```json
"attachment": {
  "name": "Mini_Task_API_TZ.md",
  "content_type": "text/plain; charset=utf-8",
  "size": 15001
}
```

### 11. Скачать вложение

```bash
curl -sS -D - -b "$COOKIE_JAR" -o /tmp/downloaded-attachment.md "$API_URL/tasks/$TASK_ID/attachment"
```

`-D -` выводит заголовки в терминал, а `-o` сохраняет только тело ответа в файл.

Ожидаемые заголовки:

```http
HTTP/1.1 200 OK
Content-Disposition: attachment; filename=Mini_Task_API_TZ.md
Content-Type: text/plain; charset=utf-8
Content-Length: 15001
```

Сравнить оригинал и скачанный файл:

```bash
cmp "$FILE_PATH" /tmp/downloaded-attachment.md
```

Если `cmp` ничего не вывел, файлы совпадают побайтово.

### 12. Выйти из аккаунта

```bash
curl -i -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X POST "$API_URL/logout"
```

Ожидаемый статус: `204 No Content`. После выхода защищённые маршруты возвращают `401 Unauthorized`.

## Как устроена работа с файлами

### Физическое хранение

Файл сохраняется без использования пользовательского имени в пути:

```text
data/attachments/{userID}/{taskID}
```

Например:

```text
data/attachments/3/7
```

Такой путь защищает хранилище от имён наподобие `../../users.json`.

### Безопасная запись

Загрузка сначала записывается во временный файл:

```text
data/attachments/3/.upload-12345
```

После успешного копирования временный файл переименовывается в итоговый через `os.Rename`. Если загрузка оборвётся, незавершённый файл не станет рабочим вложением.

### Метаданные

Байты файла находятся в `data/attachments`, а в `data/tasks.json` записываются:

- исходное имя;
- определённый MIME-тип;
- размер в байтах.

Если сохранить файл удалось, но обновление `tasks.json` завершилось ошибкой, сервис пытается удалить сохранённый файл.

### Скачивание

Файл отдаётся через `http.ServeContent`. Это даёт поддержку:

- `Content-Length`;
- `Last-Modified`;
- `HEAD`-запросов;
- range-запросов;
- условного кеширования.

Заголовок `Content-Disposition: attachment` предлагает клиенту скачать файл под исходным именем.

## Коды ответов

| Код | Значение |
|---:|---|
| `200` | Успешное чтение или вход |
| `201` | Пользователь, задача или вложение созданы |
| `204` | Успешный logout без тела ответа |
| `400` | Некорректный JSON, ID, фильтр или данные |
| `401` | Нет действующей сессии |
| `404` | Задача или вложение не найдены |
| `409` | Username занят или у задачи уже есть вложение |
| `413` | Файл превышает лимит 10 МБ |
| `500` | Внутренняя ошибка чтения или записи |

## Важные ограничения

Это учебный проект, а не готовый production-сервис:

- сессии хранятся в памяти и пропадают после перезапуска сервера;
- пользователи и задачи хранятся в JSON вместо базы данных;
- поддерживается только одно вложение на задачу;
- замена и удаление вложения через HTTP не реализованы;
- типы загружаемых файлов не ограничены;
- graceful shutdown пока отсутствует;
- автоматические тесты пока не написаны;
- глобальный mutex последовательно обрабатывает загрузки вложений;
- приложение рассчитано на один запущенный экземпляр.

После каждого перезапуска сервера необходимо снова выполнить `/login`, даже если cookie-файл сохранился: сервер уже не знает старый идентификатор сессии.

## Следующие учебные шаги

- worker pool на горутинах и каналах;
- фоновое вычисление SHA-256 вложений;
- graceful shutdown;
- тесты через `httptest`;
- удаление и замена вложений;
- переход с `http.ServeMux` на `chi`;
- замена JSON-хранилища на базу данных.
