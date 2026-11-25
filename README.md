# Тайный Санта

Небольшой самостоятельный сайт для жеребьёвки Тайного Санты.

Основное приложение находится в `santa/`: ASP.NET Core, Blazor и PostgreSQL.
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
docker compose build
docker compose up -d db
docker compose run --rm santa --migrate
docker compose up -d santa
```

Сайт будет доступен по адресу <http://localhost:5080>.

Миграция запускается отдельной командой. Сервис не меняет схему базы данных при
каждом старте.

### Как пользоваться

1. Зарегистрируйте аккаунты участников.
2. Создайте игру и добавьте email участников. Если организатор участвует,
   добавьте и его email.
3. При необходимости укажите пары исключений, по одной паре в строке.
4. Сохраните состав и запустите жеребьёвку.
5. Каждый участник войдёт под своим аккаунтом и увидит только своего получателя.
6. После закрытия игры результаты больше не открываются.

В текущей версии нет писем, подтверждения email, гостевых приглашений и
восстановления пароля. Email используется как имя аккаунта.

## Сборка без Docker

Нужны Linux, C++20 compiler, CMake 3.20 или новее, .NET SDK 10 и PostgreSQL.

```bash
git submodule update --init third_party/yasl
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j 2
ctest --test-dir build --output-on-failure
dotnet restore santa/santa.csproj --locked-mode
export ConnectionStrings__Santa='Host=localhost;Database=santa;Username=santa;Password=YOUR_PASSWORD'
dotnet run --project santa -- --migrate
ASPNETCORE_ENVIRONMENT=Development dotnet run \
  --project santa --no-launch-profile --urls http://localhost:5080
```

Сначала собирается `build/libsanta_native.so`, затем .NET-проект копирует его в
серверную сборку и в publish.

## Go library

The standalone assignment package is `santa/go/santa`. It has no HTTP or database
dependency. Build and install the native library first, then expose its pkg-config
directory and runtime library to cgo:

```bash
cmake -S . -B build-go-native -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF
cmake --build build-go-native --target santa_native -j 2
cmake --install build-go-native --prefix /tmp/santa-native
(cd go && PKG_CONFIG_PATH=/tmp/santa-native/lib/pkgconfig LD_LIBRARY_PATH=/tmp/santa-native/lib go test ./...)
```

The native ABI returns `Unsatisfiable` only for a proven impossible graph. Search,
resource, and cancellation outcomes remain distinct. The Go package accepts UUID
participants and pair exclusions; connected pairs do not receive transitive
closure. Explicit exclusion groups are not part of the current library API.

## Logging

The Go service emits structured JSON through `go.uber.org/zap`. Set `LOG_LEVEL`
to a zap level such as `info`, `warn`, or `debug`. Request logs include the
request ID, method, path, status, byte count, and duration. Publish logs include
the actor and game UUIDs, configuration version, status, and duration; passwords,
CSRF values, request keys, seeds, messages, and complete assignments are never
logged.

The native C++ adapter can emit diagnostic lines through YASL's
`yasl::io::err()` without adding a logging dependency. Set
`SANTA_NATIVE_LOG=1` to enable start/finish records containing only snapshot
sizes, budgets, status, attempts, and duration. Native C++ receives dense
snapshot-local indices, so it cannot log external participant identities.

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

Проверки включают C++ core, C ABI, отображение Guid, права доступа, гонки при
публикации, повторные запросы и откат частичной записи.

## Что где находится

- `santa/` — основной сайт;
- `core/` — C++ engine и C ABI;
- `third_party/yasl/` — исходник YASL;
- `scripts/` — smoke-проверки;
- `tests/` — интеграционный harness;
- `go/` — сохранённая параллельная реализация сервера;
- `web/` — второй шаблон, сохранённый для справки.
