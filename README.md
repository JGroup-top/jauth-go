# jauth-go

Небольшой Go SDK для JAuth: OAuth2 Authorization Code, PKCE и получение пользователя.
Без БД, HTTP-обработчиков, cookies и бизнес-логики приложений.

Требуется Go 1.27 (как в исходном проекте). Зависимость: `golang.org/x/oauth2 v0.37.0`.

## Сборка библиотеки

`jauth-go` — библиотека, а не исполняемая программа. Отдельный бинарник для
переноса в приложение не нужен: Go собирает библиотеку вместе с приложением.

Из каталога `jauth-go`:

```sh
go version
go mod download
go build ./...
go test ./...
```

Успешный `go build ./...` обычно ничего не выводит и не создаёт исполняемый
файл в каталоге библиотеки. Для этого проекта нужен Go 1.27 или новее.

Если на вашем Mac терминал отвечает `go: command not found`, SDK уже есть
в каталоге GoLand. Для текущего окна терминала добавьте его в PATH:

```sh
export PATH="$HOME/sdk/go1.27.1/bin:$PATH"
go version
```

В GoLand тот же SDK можно выбрать в настройках проекта: **Go → GOROOT**.

## Подключение к своему проекту локально

Например, каталоги расположены рядом:

```text
GolandProjects/
├── jauth-go/
│   └── go.mod
└── jdaily-backend/
    └── go.mod
```

Откройте терминал **в каталоге приложения**, где находится его `go.mod`:

```sh
go mod edit -require=jauth-go@v0.0.0
go mod edit -replace=jauth-go=../jauth-go
```

Это добавит в `go.mod` приложения:

```go
require jauth-go v0.0.0
replace jauth-go => ../jauth-go
```

Если каталоги не соседние, укажите реальный относительный или абсолютный путь
к библиотеке. Версия `v0.0.0` здесь — локальная заглушка; `replace` направляет Go
к исходникам на диске, без скачивания библиотеки с сервера.

Добавьте импорт и вызов `jauth.New` из следующего раздела **до** `go mod tidy`,
иначе Go может удалить ещё не используемую зависимость. Затем в приложении:

```sh
go mod tidy
go build ./...
go test ./...
```

Для запуска используйте обычную точку входа вашего приложения, например
`go run ./cmd/server`, если `main` расположен в `cmd/server`.
Изменения в локальной библиотеке попадут в следующую сборку приложения автоматически.
Копировать `.go`-файлы библиотеки внутрь приложения не нужно.
На CI и других компьютерах путь из `replace` также должен существовать;
для распространения используйте опубликованный модуль, как описано ниже.

## Настройка JAuth

Зарегистрируйте приложение в JAuth и получите Client ID и Client Secret.
Redirect URL в JAuth должен совпадать с адресом callback вашего приложения.
Например, для локального сервера на порту 8080 задайте в терминале приложения:

```sh
export JAUTH_CLIENT_ID='ваш-client-id'
export JAUTH_CLIENT_SECRET='ваш-client-secret'
export JAUTH_REDIRECT_URL='http://localhost:8080/auth/callback'
```

`os.Getenv` читает окружение процесса. Если храните значения в `.env`, загрузите
его средствами приложения (например, уже используемым `godotenv`) или настройте
переменные в конфигурации запуска GoLand. Сама библиотека `.env` не читает.

## Использование

```go
import (
    "context"
    "os"
    jauth "jauth-go"
    "golang.org/x/oauth2"
)

func newAuthClient() (*jauth.Client, error) {
    return jauth.New(jauth.Config{
        ClientID: os.Getenv("JAUTH_CLIENT_ID"),
        ClientSecret: os.Getenv("JAUTH_CLIENT_SECRET"),
        RedirectURL: os.Getenv("JAUTH_REDIRECT_URL"),
        // BaseURL: "http://localhost:3001", // необязательное переопределение
    })
}

// Фрагменты двух этапов входа; state и verifier нужно хранить между запросами.
func loginURL(client *jauth.Client) (loginURL, state, verifier string) {
    state = oauth2.GenerateVerifier()
    verifier = oauth2.GenerateVerifier()
    loginURL = client.AuthorizationURL(state, oauth2.S256ChallengeOption(verifier))
    return
}

// Вызывать только после проверки state и ошибки провайдера в callback.
func finishLogin(ctx context.Context, client *jauth.Client, code, verifier string) (jauth.User, error) {
    return client.Authenticate(ctx, code, oauth2.VerifierOption(verifier))
}
```

Приложение сохраняет `state` и `verifier` на сервере с привязкой к браузерной
сессии и коротким сроком жизни. В callback сначала проверяет ошибку провайдера,
наличие и совпадение `state`, одноразово извлекает сохранённый verifier, затем
вызывает `Authenticate`. Не используйте постоянное значение `state`.
Библиотека не хранит состояние входа и не проверяет callback за приложение.
PKCE требует поддержки со стороны JAuth.

Если нужно сохранить токен, используйте `Exchange(ctx, code, opts...)`, а затем
`UserInfo(ctx, token)`. `Authenticate` возвращает только `User`.
`UserInfo` автоматически обновляет просроченный токен при наличии refresh token;
полученный токен используется только в этом вызове и не записывается в исходный.
Для долговременного хранения и ротации refresh token нужна отдельная логика приложения.

## Как встроить в существующий auth-сервис

Создайте один `*jauth.Client` при запуске приложения, обработайте ошибку `New`
и передайте клиент в свой auth-сервис через конструктор. В обработчике входа
получите URL через `AuthorizationURL` и перенаправьте браузер на него.
В callback после проверки `state` вызовите `Authenticate`.

Результат содержит:

```go
type User struct {
    ID       string // sub — идентификатор пользователя в JAuth
    Username string // preferred_username
    Email    string // email
}
```

По `user.ID` приложение ищет или создаёт своего локального пользователя,
а затем создаёт собственную сессию. Эти действия остаются в вашем auth-сервисе.

Если используете интерфейс для зависимости, учтите опции PKCE в сигнатурах:

```go
type OAuthClient interface {
    AuthorizationURL(state string, opts ...oauth2.AuthCodeOption) string
    Authenticate(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (jauth.User, error)
}
```

В файле интерфейса нужны импорты `context`, `jauth "jauth-go"` и
`golang.org/x/oauth2`. Старый интерфейс без `...oauth2.AuthCodeOption`
не будет реализован этим клиентом, даже если вы вызываете методы без опций.

## Подключение через Git-репозиторий

Этот раздел нужен после публикации исходников. Сейчас module path — `jauth-go`.
Например, если ваш репозиторий будет `github.com/TheJCN/jauth-go`:

1. В библиотеке выполните `go mod edit -module=github.com/TheJCN/jauth-go`.
2. Обновите импорты самой библиотеки и примеров, если они ссылаются на `jauth-go`.
3. Проверьте сборку, опубликуйте исходники и создайте тег версии, например `v0.1.0`.
4. В приложении замените импорт на `jauth "github.com/TheJCN/jauth-go"`.
5. В приложении удалите локальную настройку и подключите опубликованную версию:

```sh
go mod edit -dropreplace=jauth-go
go mod edit -droprequire=jauth-go
go get github.com/TheJCN/jauth-go@v0.1.0
go mod tidy
go build ./...
```

Адрес и версия приведены как пример: команды `go get` заработают только после
публикации соответствующего репозитория и тега. Для приватного репозитория
также понадобятся доступ Git и настройка `GOPRIVATE`.

## Конфигурация и контракт

- `BaseURL` по умолчанию: `https://sso.jgroup.top`; завершающие `/` удаляются.
- Пути: `/oauth/authorize`, `/oauth/token`, `/userinfo`. Префикс пути в BaseURL сохраняется.
- Эти пути взяты из исходного проекта; интеграция с действующим JAuth не проверялась.
- `ClientID` и абсолютный `RedirectURL` обязательны. `ClientSecret` может быть пустым для публичного клиента с PKCE.
- Пустые `Scopes` дают `openid profile email`; непустой список заменяет их.
- Можно передать `HTTPClient` для настройки транспорта и таймаута. Клиент копируется; HTTP-перенаправления отключены для запросов токенов и UserInfo.
- Таймаут по умолчанию — 15 секунд на HTTP-запрос. Контекст позволяет ограничить весь вызов Authenticate.
- HTTP допустим для локальной разработки; рабочий сервер должен использовать HTTPS.
- UserInfo ожидается как JSON-объект с непустым строковым `sub`, необязательными `preferred_username` и `email`. Неизвестные поля игнорируются; предел ответа — 1 MiB.
- SDK использует UserInfo для получения личности пользователя, не проверяет ID token/JWT и не является полноценным OIDC-клиентом.

Ошибки конфигурации и входных параметров проверяйте через `errors.Is`.
HTTP-ошибки UserInfo доступны как `*jauth.HTTPError` через `errors.As`.
Ошибки token endpoint сохраняют `*oauth2.RetrieveError`; не выводите их полное
содержимое в публичный ответ или журнал, поскольку ответ провайдера может содержать секреты.

## Проверка

```sh
go test -race -cover ./...
go vet ./...
```

Тесты используют локальный HTTP-сервер и не требуют учётных данных JAuth.
Они проверяют настройки, PKCE, обмен кода, Authenticate, обновление токена,
ошибки, отмену контекста и некорректные ответы UserInfo.

Документация зависимости: https://pkg.go.dev/golang.org/x/oauth2
