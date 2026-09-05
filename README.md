# MitoBoat

A multi-tenant Twitch chat bot: one process serves every registered streamer's
channel from a single IRC connection.

## Setup

Create a Twitch application at https://dev.twitch.tv/console/apps and add
`http://localhost:8080/auth/callback` to its OAuth Redirect URLs.

```sh
cp .env.example .env               # fill in the credentials
openssl rand -base64 24            # use this as ADMIN_SECRET

go run ./cmd/mitoboat -s           # create the schema
go run ./cmd/mitoboat -a           # authorization server only
```

Open `http://localhost:8080/auth/bot?key=$ADMIN_SECRET` and sign in **as the
bot account**. That stores the token the bot posts as, which is the one thing
it cannot start without. Then:

```sh
go run ./cmd/mitoboat               # start the bot
```

Streamers add the bot themselves at `http://localhost:8080/` — no admin key,
and their channel is joined within seconds, without a restart.

| Flag | Meaning |
| ---- | ------- |
| `-s` | Run the database migration and exit |
| `-a` | Run only the authorization server, without joining chat |
| `-v` | Log every SQL statement |

## Running with Compose

`docker-compose.yml` brings up PostgreSQL, runs the migration, and starts the
bot. It reads the same `.env` as a local run, so fill that in first; `DB_HOST`,
`DB_PORT` and `HTTP_ADDR` are overridden for the container network and can be
left at their defaults.

First run, to store the bot token:

```sh
docker compose --profile bootstrap up auth   # postgres, migrate, then -a
```

Open `http://localhost:8080/auth/bot?key=$ADMIN_SECRET`, sign in **as the bot
account**, then stop it with Ctrl-C. After that:

```sh
docker compose up -d                         # postgres, migrate, bot, adminer
docker compose logs -f bot
```

| Service | Role |
| ------- | ---- |
| `postgres` | The database, on a named `pgdata` volume |
| `migrate` | Runs `mitoboat -s` and exits; the bot waits for it to succeed |
| `bot` | Chat plus the authorization flow |
| `auth` | `mitoboat -a`, in the `bootstrap` profile, for the first-run token |
| `adminer` | Web UI for the database, on `http://127.0.0.1:8081` |

Adminer replaces `psql` inside the container: log in with `postgres` as the
server (already prefilled) and the `DB_USER` / `DB_PSSWD` / `DB_NAME` from
`.env`. It is published on loopback only, because it reaches the database with
full privileges — do not bind it to a public interface. `ADMINER_PORT` moves it
off 8081.

Set `HTTP_PORT` to publish on a different host port, and `VERSION` to stamp the
binary. `podman-compose` works the same way.

### Authorization

| Route | Access |
| ----- | ------ |
| `/` | Public: the page a streamer starts from |
| `/auth/streamer` | Public. Authorizing grants access to that streamer's own channel only |
| `/auth/bot` | Requires `?key=$ADMIN_SECRET`: this token lets the bot speak in **every** channel |
| `/auth/callback` | Twitch redirects here; protected by a single-use, expiring state token |

Changing the bot token takes effect on restart, because IRC authenticates once
at connect time. Streamer registrations take effect immediately.

Configuration is read from the environment, falling back to a `.env` file.
Every variable is documented in [`.env.example`](.env.example).

`SIGINT` and `SIGTERM` shut the bot down cleanly: the IRC connection is closed,
the background refreshers stop, and the connection pool is drained.

## Layout

```
cmd/mitoboat        entry point: flags, logger, signal handling
internal/config     environment configuration and its validation
internal/domain     persisted entities, and nothing else
internal/store      the database connection and every query
internal/commands   command parsing and the in-memory command cache
internal/twitch     Helix clients, OAuth, IRC, and chat rate limiting
internal/web        the authorization server
internal/bot        wiring, the streamer registry, and the message handler
internal/flags      command line parsing
```

Dependencies point inwards: `domain` imports nothing but GORM, `store` and
`twitch` depend on `config` and `domain`, and only `bot` knows about all of
them. Nothing outside `store` writes a query, and nothing outside `twitch`
talks to Twitch.

## How it scales to many streamers

The bot joins every active streamer from one IRC connection, and
`go-twitch-irc` dispatches messages **inline on the single goroutine that reads
that connection**. Anything slow in the message handler therefore delays chat
for every channel at once, which drives most of the design:

- **Commands are served from memory.** A `commands.Cache` holds the global and
  per-streamer command tables and is rebuilt on a ticker
  (`COMMAND_CACHE_TTL`). Answering a command is two map reads under a read
  lock and never touches the database.
- **Streamer lookup is a map, not a scan.** `bot.Registry` indexes streamers by
  id and by lowercased username, both pointing at the same `*StreamerContext`,
  so a token refreshed in the background is visible to the handler.
- **Outbound chat is rate limited per channel.** Twitch counts messages per
  *account*, so exceeding the limit silences the bot everywhere. A token bucket
  per channel (`SAY_BURST` / `SAY_WINDOW`) keeps one busy channel from spending
  everyone else's budget. `go-twitch-irc` rate limits JOINs but not messages.
- **Commands have a per-channel cooldown** (`COMMAND_COOLDOWN`), which also
  breaks feedback loops between bots. The bot additionally ignores its own
  messages.

Memory stays flat as streamers are added: the connection pool is bounded
(`DB_MAX_OPEN_CONNS`), every Helix client shares one explicitly sized HTTP
transport instead of `http.DefaultClient`, and the cooldown and rate-limiter
maps are pruned rather than growing per channel seen.

## Tokens

Twitch user tokens last about four hours and application tokens about sixty
days, so both are refreshed in the background rather than only at startup. A
refreshed streamer token is written back to the database immediately.

A streamer whose token cannot be validated is still joined and still gets text
commands; only the Helix API is unavailable for them.

## Tests

```sh
go test -race ./...
```

The `internal/store` tests exercise the SQL against a real PostgreSQL and skip
when none is configured. To run them:

```sh
podman run -d --rm --name mitoboat-test \
  -e POSTGRES_PASSWORD=testpass -e POSTGRES_USER=mitoboat -e POSTGRES_DB=mitoboat \
  -p 55432:5432 docker.io/library/postgres:16-alpine

DB_HOST=127.0.0.1 DB_PORT=55432 DB_NAME=mitoboat DB_USER=mitoboat DB_PSSWD=testpass \
  go test -race -count=1 ./internal/store/
```
