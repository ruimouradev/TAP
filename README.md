*This project has been created as part of the 42 curriculum by rusilva-, acaldeir.*

# The Answer Protocol

## Description

A TCP-based multi-user dungeon (MUD) where multiple players explore a shared world in real time. The server implements the RFC 42TAP protocol: players connect, move between rooms, pick up items, fight NPCs, and complete quests — all simultaneously. Two clients are provided: a terminal CLI and a Fyne-based desktop GUI.

Built in Go. No persistence — world state resets when the server restarts.

## Instructions

> See the **Building and Running** section for details on each component.

```
make deps           # download Go dependencies
make run-server     # start the TAP server (requires data/world.json)
make run-client     # start the CLI client (connects to localhost:4300)
make run-client-gui # start the Fyne GUI client (connects to localhost:4300)
make lint           # run gofmt + go vet
make clean          # remove build artifacts
```

---

## Architecture

The server uses a **Hub/Actor** pattern: a single goroutine (the Hub, `internal/server/hub.go`) owns all mutable world state. Client goroutines communicate with the Hub exclusively via channels — they never touch the world directly. Because the Hub processes one event at a time, race conditions on game state are impossible **without a single mutex**. (The alternative — guarding every `World`/`Room`/`Player` access with locks — is easy to get subtly wrong and hard to reason about; the actor model trades that for one clear ownership rule.)

Each connected client runs two goroutines:
- **readPump** — reads lines from the TCP socket, parses them, and forwards `Command` structs to the Hub over the `commands` channel.
- **writePump** — drains a buffered `send` channel and writes responses/events to the socket.

The Hub's `select {}` loop handles `register` / `unregister` / `command` events one at a time. Commands are routed through a **metadata-driven table** (`internal/server/dispatch.go`): each verb maps to a `Command{Handler, MinArgs, Anon, Usage}` spec, so the dispatcher enforces the authenticated-client check and the argument-count check in one place and the handlers stay thin (it also resolves the player once and passes it in). Since handlers run inside the Hub goroutine, they read and mutate `h.world` directly with no locking. Abuse monitoring (command flooding per client, rapid connections per IP) shares one sliding-window counter (`internal/server/ratelimit.go`), and wire errors are typed values (`protocol.Error{Code, Msg}`) so a code can never drift from its message.

Back-pressure is handled by `safeSend`: the per-client `send` channel is buffered, and if it ever fills (a client too slow to drain) the Hub drops that connection instead of blocking — one slow client can never stall the whole server. Shutdown is graceful via `signal.NotifyContext` (Ctrl+C stops accepting and exits cleanly).

Layering: `internal/protocol` is the wire contract (parsing + formatting, no game logic), `internal/game` holds the world rules (no networking), and `internal/server` glues the two — handlers translate protocol ↔ game. This keeps the game logic unit-testable without a socket.

---

## Protocol Implementation

This implementation follows RFC 42TAP. Messages are line-based (`\n`-terminated, UTF-8). Known deviations:

| Command | RFC / V.5 example | Our response | Reason |
|---------|-------------------|--------------|--------|
| `WHO`   | `OK players=<count>` | `OK {"room":[...],"server":<n>}` | Subject examples show richer format |
| `TALK`  | `OK <dialogue>` | `OK {"npc":"...","dialogue":"..."}` | Subject examples show JSON format |
| `LOOK` (items) | `"items":["item.herbs"]` | `"items":[{"id":"item.herbs","name":"Healing Herbs"}]` | GUI needs the display name to render the room without a separate lookup; avoids a fragile client-side ID→name cache |
| `LOOK` (npcs)  | `"npcs":["npc.baker"]` | `"npcs":[{"id":"npc.baker","name":"Baker","hostile":false}]` | Same as above, plus `hostile` flag so the GUI can colour enemies differently |
| `INVENTORY` | `OK ["item.herbs"]` | `OK [{"id":"item.herbs","name":"Healing Herbs"}]` | Same reasoning: GUI needs names to display inventory |

All deviations remain ABNF-compliant. The RFC's grammar is `response-line = ("OK" / error-response) [SP response-data] LF`, and `response-data` is referenced but **never defined** — so the grammar does not constrain what follows `OK `, and any JSON payload conforms. The subject's own examples even deviate from the ABNF (they show spaces inside JSON, which a stricter reading would forbid), confirming the examples are illustrative rather than literal specifications.

For interoperability with other groups' servers that follow the V.5 example formats, the GUI parses both shapes (plain ID string OR `{id, name}` object) inside `cmd/gui/main.go` (in the LOOK/INVENTORY response handlers) and normalises them before rendering.

**Extra room events.** Beyond the RFC's event list, the server broadcasts two extra room-scoped events, **only to players in the affected room**, so the shared world stays live without clients polling:
- `EVT ROOM COMBAT <attacker> <target> <damage> <target_hp>` — others in the room see a fight as it happens.
- `EVT ROOM ITEM TAKEN <player> <item_id>` / `EVT ROOM ITEM DROPPED <player> <item_id>` — others' room view updates instantly when an item is picked up or dropped.

Both follow the standard `EVT <category> <type> [data]` shape, are scoped to the room (never leaked to other recipients), and are safely ignored by any client that does not recognise them.

**Error code extension.** The RFC (§8.2) defines codes for game errors (201, 301, 401, 402, 404, 405, 406), which we follow exactly. For input the RFC does not define — unknown verb, command before `CONNECT`, missing arguments — we return `ERR 400 <reason>` (e.g. `UNKNOWN_COMMAND`, `NOT_CONNECTED`, `MISSING_ITEM`). `400` is our extension; the RFC only states malformed messages should yield an appropriate error.

**Line handling.** Each connection is read with a `bufio.Scanner` whose buffer starts at 64&nbsp;KB and grows up to a **1&nbsp;MB** maximum line length, so very long (but valid) lines are accepted while a single unbounded line cannot exhaust memory. The protocol tolerates `CRLF` (a trailing `\r` is trimmed) and ignores blank lines.

---

## Combat System

Players start at **100 HP**. Combat lives in `internal/game/combat.go`.

- **Turn model**: each `ATTACK <npc>` command resolves **one round** — there is no persistent "in combat" state, so any other command is valid between rounds. This keeps the server stateless per-action and avoids tracking combat sessions or timeouts.
- **Damage formula**: each hit deals a uniform random amount in **`[10, 20]`** (`calculateDamage()`). Players and NPCs have no separate attack/defense stats, so the formula is deliberately simple; difficulty is tuned by changing the range (`minDamage`/`maxDamage`).
- **Initiative**: the player always strikes first. If the NPC survives, it **counter-attacks** in the same round (also `[10, 20]`); if the NPC is defeated, there is no counter-attack.
- **Targets**: only **hostile** NPCs can be attacked — attacking a non-hostile NPC returns `ERR 405 NPC_NOT_HOSTILE`; an absent NPC returns `ERR 404 NPC_NOT_FOUND`.
- **Defeat & respawn**: when a player's HP reaches 0 they **respawn at the start room with 50% of MaxHP** (`RespawnPlayer`). The defeat is reported in the `ATTACK` response (`"status":"defeat"`); the client refreshes HP with `STATUS`.
- **Extra commands**: none — `DEFEND`/`FLEE` were considered but left out to keep the system minimal (not required by the RFC).
- The `ATTACK` response is JSON: `{damage, counter, attacker_hp, target_hp, status}` where `status` is `combat` | `victory` | `defeat`.

---

## Quest System

Quests live in `internal/game/quest.go`. Two types are implemented (the RFC lists `deliver` too, which we did not implement — two distinct types is the requirement):

- **`fetch`** — objective met when the target item is in the player's inventory.
- **`defeat`** — objective met when the player has defeated the target NPC.

**State per player.** Quest *definitions* are shared (`World.Quests`, loaded from `world.json`), but each player keeps their own progress (`Player.Quests`), with the state machine `NotStarted → Active → Completed` (not present in the map = NotStarted).

**Flow.**
- `QUEST <npc>` asks a quest-giver NPC for its quest. If the NPC has one and the player hasn't already started/finished it, the quest becomes `Active`; otherwise `ERR 406 NO_QUEST_AVAILABLE`.
- `QUESTS` lists the player's quests. **Completion is checked lazily here**: any active quest whose objective is now met is marked `Completed` and its reward granted at that moment. (Lazy checking avoids watching the inventory/combat in real time — we validate when the player asks.)

**Defeat tracking.** To validate `defeat` objectives per player, the combat code records each kill in `Player.Defeated` (a set of npcIDs); `defeat` completion simply checks that set — so a kill by another player doesn't complete your quest.

**Rewards.** The reward is an item ID. On completion it is copied from the world item catalog (`World.Items`) into the player's inventory (with its proper display name).

World quests: `quest.fetch_herbs` (from the Baker → reward `item.herbalist_token`, *Herbalist's Token*) and `quest.defeat_goblin_chief` (from the Goblin Chief → reward `item.chief_trophy`, *Goblin Chief's Trophy*). Reward items are **dedicated** — they are not placed anywhere in the world, so a granted reward never duplicates an item instance that already exists in a room.

---

## World Design

The TAP world consists of 8 distinct rooms designed with a **circular loop structure** to encourage exploration and player interaction, with a few dangerous branches.

### Map Layout

```text
         Bakery       Forest Path ──────┐
           |               |            |
  Inn ── Village ─────── Market         |
         Square                         |
           |                            |
          Gate                          |
           |                            |
         Forest ────────────────────────┘
        Entrance
           |
      Deep Forest
```

NPC roles (7 NPCs): **guard** (Guard — dialogue), **merchant** (Baker, Merchant, Innkeeper), **entertainer** (Bard — dialogue), **enemy** (Goblin, Goblin Chief). Quest-givers: Baker (`fetch`) and Goblin Chief (`defeat`).

Items (8): Loaf of Bread, Healing Herbs, Rusty Sword, Ancient Relic, Silver Amulet, Torn Letter — all placed in rooms and obtainable with `TAKE` — plus two **dedicated quest rewards** (Herbalist's Token, Goblin Chief's Trophy) that exist only as rewards, never in a room.

Quests: `fetch_herbs` (Baker), `defeat_goblin_chief` (Goblin Chief).

---

## Server Logging

Uses Go's `log/slog` with a **JSON handler**, written to **stdout** (redirect with `> server.log`). Every line carries an automatic `time` and a `level`. One line per event makes the log easy to filter (`grep`/`jq`).

| Event | `msg` | Level | Fields |
|-------|-------|-------|--------|
| Connection | `client connected` | INFO | `addr` |
| Disconnection | `client disconnected` | INFO | `addr`, `user` |
| Command received | `command` | INFO | `user`, `verb`, `args` |
| Response sent | `response` | INFO / **WARN** if `ERR` | `user`, `verb`, `resp` |
| Unknown verb | `unknown command` | WARN | `user`, `verb` |
| Item moved | `item taken` / `item dropped` | INFO | `user`, `item`, `room` |
| Combat result | `combat` | INFO | `user`, `target`, `damage`, `counter`, `status` |
| Group change | `group created/joined/left` | INFO | `user`, `group` |
| Quest event | `quest started` / `quest completed` | INFO | `user`, `quest`, `reward` |
| Abuse (flooding) | `possible command flood` | **WARN** | `user`, `addr`, `count`, `window` |
| Abuse (rapid conns) | `possible rapid connections` | **WARN** | `addr`, `count`, `window` |
| Startup / fatal | `server listening`, `load world`, … | INFO / ERROR | `addr`, `err` |

**Log levels**: INFO for normal activity, **WARN** for error responses and abuse, **ERROR** for fatal startup failures.

**Abuse detection**: we *monitor and log* rather than disconnect (the spec asks for monitoring). Two patterns are detected:
- **Command flooding** (per client, `c.cmdRate` in `internal/server/dispatch.go`): each client's commands are counted in a sliding **1-second window**; more than **20 commands** in a window logs one `possible command flood` WARN.
- **Rapid connections** (`trackRapidConnections`, `internal/server/hub.go`): connections are counted per IP in a sliding **10-second window**; more than **5 connections** from one IP logs one `possible rapid connections` WARN.

Example output:
```json
{"time":"2026-06-17T00:43:43Z","level":"INFO","msg":"client connected","addr":"127.0.0.1:57164"}
{"time":"2026-06-17T00:43:43Z","level":"INFO","msg":"command","user":"alice","verb":"MOVE","args":["north"]}
{"time":"2026-06-17T00:43:43Z","level":"WARN","msg":"response","user":"alice","verb":"TAKE","resp":"ERR 404 ITEM_NOT_FOUND"}
```

---

## Group Contributions

| Component | Owner |
|-----------|-------|
| TCP server, Hub, concurrency model | rusilva- |
| Command dispatcher + all handlers | rusilva- |
| Item system, combat, quest logic | rusilva- |
| CLI client | rusilva- |
| `internal/protocol` package | Both |
| GUI client (Fyne, `cmd/gui`) | acaldeir |
| World data (`data/world.json`) | acaldeir |
| World validator (`internal/worldfile`) | acaldeir |
| Makefile + build tooling | acaldeir |
| README | Both |

---

## Building and Running

### Requirements

- Go 1.22+
- `make`
- A C compiler (`clang` or `gcc`) — Fyne uses CGO
- On Linux: graphics dev headers (`xorg-dev` + `libgl1-mesa-dev` on Debian/Ubuntu)

### Steps

```bash
# 1. Install Go module dependencies
make deps

# 2. Start the server (reads data/world.json)
make run-server

# 3. In a new terminal — CLI client
make run-client

# 4. Or — Fyne GUI client (opens its own window)
make run-client-gui
```

To run with the race detector (recommended during development):

```bash
go run -race ./cmd/server data/world.json
```

---

## Testing

### Automated tests

Run from the project root (the race detector catches concurrency bugs):

```bash
go test -race ./...        # all packages
go test -race ./internal/server -v   # server integration tests, case by case
go test -fuzz=FuzzParse ./internal/protocol   # fuzz the command parser
```

The suite covers the protocol (parsing, responses, events), the game core
(world, items, combat, quests) and the server end-to-end over real TCP
(vertical slice, presence/chat, items, combat, groups, quests, duplicate names).
The command parser is also **fuzz-tested** (`FuzzParse`): Go generates thousands
of malformed lines and checks `Parse` never panics and that any accepted line has
a non-empty, upper-cased verb.

### Network & buffering (manual)

To verify that the server's TCP parser handles buffering correctly (including multiple commands sent in a single TCP packet or split packets), you can write raw commands directly to the server using `netcat` (`nc`):

```bash
# Send multiple commands inside a single TCP connection/packet:
echo -e "CONNECT evaluator\nLOOK\nWHO" | nc localhost 4300
```

The server should process each command sequentially, replying with the respective protocol responses (e.g., `OK connected`, the room JSON state, and the server activity JSON) before closing the connection.

### Multiplayer (manual)

Start the server, then open two terminals with `make run-client` and connect with
different usernames. Verify:
- `MOVE` broadcasts `EVT ROOM PRESENCE ENTER/LEAVE` to players in the room left/entered
- `CHAT ROOM` reaches only the same room; `CHAT GLOBAL` reaches everyone
- `Unicode & Encoding`: Send an emoji in a chat message `CHAT GLOBAL Hello 🌍` and verify it renders properly in both CLI and GUI clients without encoding errors
- `Control Characters`: Send a chat message with a control character (e.g., `\x07` BEL) via `echo -e "CONNECT alice\nCHAT GLOBAL Hello\x07World" | nc localhost 4300` and verify that another connected client receives `EVT GLOBAL CHAT alice HelloWorld` (verifying that the control character is safely stripped by the server)
- one player's `TAKE` removes the item from the room for the other
- a group: `GROUP CREATE` / `GROUP INVITE <name>` / `GROUP JOIN` / `CHAT GROUP <msg>`

### Combat

```
CONNECT alice
MOVE east          # square -> market
MOVE north         # market -> forest_path (a hostile Goblin is here)
ATTACK Goblin      # repeat until "status":"victory"
ATTACK Goblin      # now ERR 404 NPC_NOT_FOUND — the defeated NPC is gone
STATUS             # HP reduced by the counter-attacks
```

To see a respawn, fight the stronger Goblin Chief (`loc.deep_forest`) until your
HP hits 0 — you respawn at the start room with half HP.

### Quests

```
CONNECT alice
MOVE north         # square -> bakery
QUEST Baker        # accept quest.fetch_herbs (active)
QUESTS             # shows "status":"active"
MOVE south
MOVE east          # -> market
TAKE Healing Herbs # the quest objective
QUESTS             # now "status":"completed" — reward (Herbalist's Token) granted
INVENTORY          # contains item.herbalist_token
```

---

## Resources

- [The Go Programming Language — Donovan & Kernighan](https://www.gopl.io) — chapter 8 (concurrent chat server)
- [A Tour of Go](https://go.dev/tour) — goroutines and channels
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Concurrency Patterns — Rob Pike](https://go.dev/talks/2012/concurrency.slide)
- [RFC 42TAP](rfc/protocol-rfc.html) — project protocol specification

### AI Usage

AI was used to support the learning process, specifically for:
- Clarifying the project requirements and understanding the scope of the RFC 42TAP specification
- Answering questions about Go concepts such as goroutines, channels, and the concurrency model
- Discussing and evaluating different approaches to dividing the work between team members
- Assisting with debugging by explaining error messages and suggesting possible causes
- Structuring and drafting the README