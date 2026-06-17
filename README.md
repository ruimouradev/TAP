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
make run-client     # start the CLI client (connects to localhost:4242)
make run-client-gui # start the Fyne GUI client (connects to localhost:4242)
make lint           # run gofmt + go vet
make clean          # remove build artifacts
```

---

## Architecture

The server uses a **Hub/Actor** pattern: a single goroutine (the Hub, `internal/server/hub.go`) owns all mutable world state. Client goroutines communicate with the Hub exclusively via channels — they never touch the world directly. Because the Hub processes one event at a time, race conditions on game state are impossible **without a single mutex**. (The alternative — guarding every `World`/`Room`/`Player` access with locks — is easy to get subtly wrong and hard to reason about; the actor model trades that for one clear ownership rule.)

Each connected client runs two goroutines:
- **readPump** — reads lines from the TCP socket, parses them, and forwards `Command` structs to the Hub over the `commands` channel.
- **writePump** — drains a buffered `send` channel and writes responses/events to the socket.

The Hub's `select {}` loop handles `register` / `unregister` / `command` events one at a time. Commands are routed through a `verb → HandlerFunc` map (`internal/server/dispatch.go`); since handlers run inside the Hub goroutine, they read and mutate `h.world` directly with no locking.

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

All deviations remain ABNF-compliant: the RFC's `response-data` production is `1*VCHAR`, so any JSON payload is valid. The V.5 examples in the subject are illustrative — the same examples already deviate from the RFC for `TALK` and `WHO`, confirming they are not literal specifications.

For interoperability with other groups' servers that follow the V.5 example formats, the GUI parses both shapes (plain ID string OR `{id, name}` object) inside `cmd/gui/main.go` (in the LOOK/INVENTORY response handlers) and normalises them before rendering.

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

World quests: `quest.fetch_herbs` (from the Baker → reward `item.bread`) and `quest.defeat_goblin_chief` (from the Goblin Chief → reward `item.rusty_sword`).

---

## World Design

> [!WARNING]
> **TODO (acaldeir)** — Describe the full room layout, loop structure, NPC roles and their behaviour, and item distribution across rooms.

8 rooms arranged in a loop with one branch. Map overview:

```
Village Square ── Bakery
     |    |
   Inn   Market ── Forest Path
     |                   |
   Gate            Deep Forest
     |
 Forest Entrance ──────────────┘
```

NPC roles: merchant (Baker, Merchant), guard (Guard), enemy (Goblin, Goblin Chief).

Items: Loaf of Bread, Healing Herbs (obtainable), Rusty Sword (obtainable), Ancient Relic.

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
| Startup / fatal | `server listening`, `load world`, … | INFO / ERROR | `addr`, `err` |

**Log levels**: INFO for normal activity, **WARN** for error responses and abuse, **ERROR** for fatal startup failures.

**Abuse detection** (`trackFlood`, `internal/server/dispatch.go`): each client's commands are counted in a sliding **1-second window**; more than **20 commands** in a window logs one `possible command flood` WARN. We *monitor and log* rather than disconnect (the spec asks for monitoring). Rapid reconnections are observable through the `client connected` lines (each carries the IP and timestamp); automatic per-IP reconnection detection is not implemented.

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

> [!WARNING]
> **TODO (both)** — Expand these sections once the implementation is complete. Include exact commands, expected outputs, and edge cases tested.

### Multiplayer

Open two terminals and run `make run-client` in each. Connect with different usernames and verify:
- `MOVE` broadcasts `EVT ROOM PRESENCE ENTER/LEAVE` to the other player
- `CHAT ROOM` is visible only to players in the same room
- `CHAT GLOBAL` is visible to everyone
- One player taking an item removes it from the room for all others

### Combat

```
CONNECT alice
MOVE south        # navigate to forest
ATTACK goblin
STATUS            # verify HP changed
```

Kill the goblin and verify it disappears. Reduce your HP to 0 and verify respawn.

### Quests

```
CONNECT alice
MOVE east         # go to market, pick up herbs
TAKE Healing Herbs
MOVE west         # back to square
MOVE north        # to bakery
QUEST baker       # accept quest
TALK baker        # trigger completion check
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
- Structure and dratf the readme