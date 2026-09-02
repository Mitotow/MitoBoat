# MitoBoat

A multi-tenant Twitch chat bot: one process serves every registered streamer's
channel from a single IRC connection.

## Running

```sh
cp .env.example .env      # then fill in the credentials
go run ./cmd/mitoboat -s  # create or update the schema, then exit
go run ./cmd/mitoboat     # start the bot
```

| Flag | Meaning |
| ---- | ------- |
| `-s` | Run the database migration and exit |
| `-v` | Log every SQL statement |

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
