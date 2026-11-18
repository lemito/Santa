# Тайный Санта

Небольшой самостоятельный сайт для жеребьёвки Тайного Санты.

Основное приложение находится в `go/`: Go и PostgreSQL. Предыдущая версия на
ASP.NET Core / Blazor сохранена в `santa/` и запускается через профиль `dotnet`.
Жеребьёвка выполняется нативным C++20 модулем через C ABI. YASL подключается
как Git submodule в `third_party/yasl`.

Правила простые:

- каждый участник дарит одному человеку и получает подарок от одного человека;
- себя выбрать нельзя;
- исключение `A — B` действует в обе стороны;
- взаимное дарение для остальных пар разрешено.

## Запуск через Docker

Нужны Git, Docker и Docker Compose.

```bash
git submodule update --init third_party/yasl
cp .env.example .env
# Задайте в .env случайное значение POSTGRES_PASSWORD.
docker compose build santa-go
docker compose up -d db
docker compose run --rm santa-go --migrate
docker compose up -d santa-go
```

Сайт будет доступен по адресу <http://localhost:5080>.

Миграция запускается отдельной командой. Для запуска сохранённой .NET-версии
используйте `docker compose --profile dotnet up -d santa` после её миграции.

### Как пользоваться

1. Зарегистрируйте аккаунты участников.
2. Создайте игру и добавьте email участников. Если организатор участвует,
   добавьте и его email.
3. При необходимости укажите пары исключений, по одной паре в строке.
4. Сохраните состав и запустите жеребьёвку.
5. Участник может оставить своему Санте сообщение с пожеланиями, а после
   жеребьёвки Санта может анонимно ответить подопечному.
6. В сообщениях поддерживается безопасный Markdown, включая изображения по
   `http`- и `https`-ссылкам. Каждый участник видит только своего получателя.
7. После закрытия игры результаты больше не открываются.

В текущей версии нет писем, подтверждения email, гостевых приглашений и
восстановления пароля. Email используется как имя аккаунта.

## Сборка без Docker

Нужны Linux, C++20 compiler, CMake 3.20 или новее, Go 1.23 и PostgreSQL.

```bash
git submodule update --init third_party/yasl
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j 2
ctest --test-dir build --output-on-failure
cd go
go mod download
export DATABASE_URL='postgres://santa:YOUR_PASSWORD@localhost:5432/santa?sslmode=disable'
export SANTA_GO_ADDR=:5080
export SANTA_GO_SECURE_COOKIES=false
LD_LIBRARY_PATH=../build go run ./cmd/santa-go --migrate
LD_LIBRARY_PATH=../build go run ./cmd/santa-go
```

Сначала собирается `build/libsanta_native.so`, затем Go-сервис загружает его
через C ABI.

## Проверки

Для полного локального прогона нужна отдельная тестовая база PostgreSQL:

```bash
export SANTA_TEST_DB='Host=localhost;Database=santa_test;Username=santa;Password=YOUR_PASSWORD'
bash scripts/check.sh
```

HTTP smoke запускается против уже работающего сайта:

```bash
SANTA_TEST_URL=http://localhost:5080 python3 scripts/http-smoke.py
```

Проверки включают C++ core, C ABI, отображение Guid, права доступа, безопасный
Markdown сообщений, гонки при публикации, повторные запросы и откат частичной
записи.

## Что где находится

- `go/` — основной сайт на Go;
- `santa/` — сохранённая версия сайта на ASP.NET Core / Blazor;
- `core/` — C++ engine и C ABI;
- `third_party/yasl/` — исходник YASL;
- `scripts/` — smoke-проверки;
- `tests/` — интеграционный harness;
- `web/` — второй шаблон, сохранённый для справки.
