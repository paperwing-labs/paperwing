# Paperwing

Paperwing provides one private, unified inbox and read-only API for multiple IMAP accounts.

The first MVP is intentionally small: a single-user, self-hosted service that
connects multiple mailboxes through IMAP, listens for new mail in near real
time, and exposes received mail through one API.

## How it works

The mail ingestion path and API path are deliberately decoupled:

```text
IMAP IDLE -> catch-up sync -> MIME parser -> SQLite + attachment volume
                                              |
HTTP API  ------------------------------------+
```

One goroutine owns each mailbox connection. An `EXISTS` notification ends
`IDLE` and triggers an incremental UID sync. A disconnected mailbox reconnects
with exponential backoff and always runs a catch-up sync before returning to
`IDLE`, so notifications are hints rather than the source of truth. The API
only reads persisted data and never waits on IMAP, except for the explicit
manual sync endpoint.

Messages are deduplicated by `(account_id, UIDVALIDITY, UID)`. Passwords are
encrypted at rest with AES-256-GCM using an automatically managed master key.
Attachments and downloaded messages are streamed through files instead of
being buffered whole in memory.

## Run with Docker Compose

Start the service:

```sh
docker compose up --build -d
curl http://127.0.0.1:8080/healthz
```

Then open [http://127.0.0.1:8080](http://127.0.0.1:8080) to use the Paperwing web app. You can add, test, sync, and manage IMAP accounts from the interface, read messages, and download attachments.

There is no required first-run configuration. Paperwing generates a random
32-byte master key at `/data/master.key` inside the persistent volume and
reuses it on every subsequent start.

The included Compose configuration listens on all host interfaces so Paperwing
can be opened from a trusted local network. On first launch, the web app asks
you to create a single administrator account. Passwords are protected with
Argon2id and authenticated sessions expire after 30 days.

Complete the first-run administrator setup before exposing the service outside
a trusted network. Port 8080 serves plain HTTP, so public deployments still
need an HTTPS reverse proxy such as Caddy or nginx. Do not send the administrator
password over an untrusted plain-HTTP connection.

## Authentication

The web app uses the `paperwing_session` HTTP-only cookie. API clients and
personal assistants should use a revocable API token instead of storing the
administrator password. Create one from the key icon in the web app, copy it
when it is shown, and send it as a bearer token:

```sh
curl -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" \
  http://127.0.0.1:8080/accounts
```

Store the token in the client's secret manager rather than in source files or
shell history. The full value is displayed only once. Tokens can be named,
given an expiration date, inspected by last-use time, and revoked immediately.
Available scopes are:

| Scope | Allows |
|---|---|
| `mail:read` | List and read messages and download attachments |
| `accounts:read` | List configured mailboxes and monitoring states |
| `accounts:write` | Test and add mailboxes |
| `sync:write` | Request an immediate mailbox synchronization |

Browser sessions have full access. API token creation, listing, and revocation
always require the administrator's browser session and cannot be performed by
another API token. `/healthz` and the authentication bootstrap endpoints remain
public.

The first administrator can also be created through `POST /auth/setup`, but
the web setup screen is recommended. Setup is permanently disabled after the
administrator has been created.

## Complete API example

Most providers require an app password when two-factor authentication is
enabled. First, test the connection without saving credentials:

```sh
curl -X POST http://127.0.0.1:8080/accounts/test \
  -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Personal",
    "host": "imap.example.com",
    "port": 993,
    "tls": true,
    "username": "me@example.com",
    "password": "app-password"
  }'
```

Save the account using the same request body:

```sh
curl -X POST http://127.0.0.1:8080/accounts \
  -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Personal",
    "host": "imap.example.com",
    "port": 993,
    "tls": true,
    "username": "me@example.com",
    "password": "app-password"
  }'
```

The response contains an account ID. Watch `monitor_status` until it becomes
`idle`, then list and read messages:

```sh
curl -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" http://127.0.0.1:8080/accounts
curl -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" 'http://127.0.0.1:8080/emails?page=1&page_size=50'
curl -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" http://127.0.0.1:8080/emails/EMAIL_ID
curl -OJ -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" http://127.0.0.1:8080/emails/EMAIL_ID/attachments/ATTACHMENT_ID
```

Filter by source mailbox or request an immediate catch-up sync:

```sh
curl -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" 'http://127.0.0.1:8080/emails?account_id=ACCOUNT_ID'
curl -X POST -H "Authorization: Bearer ${PAPERWING_API_TOKEN}" http://127.0.0.1:8080/accounts/ACCOUNT_ID/sync
```

See [openapi.yaml](./openapi.yaml) for the complete contract.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PAPERWING_ADDRESS` | `127.0.0.1:8080` | HTTP listen address |
| `PAPERWING_DATABASE_PATH` | `./data/paperwing.db` | SQLite database path |
| `PAPERWING_ATTACHMENT_PATH` | `./data/attachments` | Attachment storage directory |
| `PAPERWING_SHUTDOWN_SECONDS` | `15` | Graceful shutdown timeout |

Back up the complete data volume, which contains the database, attachments,
and `master.key`. Existing mailbox passwords cannot be decrypted if the key is
lost. Paperwing creates and reuses this key automatically; no key configuration
is required.

## Local development

Go 1.23 and Node.js 22 or newer are required. Run the API in one terminal:

```sh
go run ./cmd/paperwing
```

Then run the Vite development server in another terminal. API requests are proxied to port 8080:

```sh
cd frontend
npm install
npm run dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). For a production-style local build that is embedded into the Go binary, run `npm run build` before `go build`.

```sh
go test ./...
cd frontend && npm run build
```

Local development automatically creates and reuses `./data/master.key`.

Only implicit TLS (`tls: true`) and explicitly unencrypted IMAP (`tls: false`)
are supported in this MVP. STARTTLS is not currently exposed by the account
model.
