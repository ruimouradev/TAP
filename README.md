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

> [!WARNING]
> **TODO (rusilva-)** — Explain the Hub/Actor concurrency model vs the mutex approach, the goroutine-per-client design, the dispatcher map, and why no mutexes are needed on the World structs.

The server uses a Hub/Actor pattern: a single goroutine owns all mutable world state (`internal/server/hub.go`). Client goroutines communicate with the Hub exclusively via channels — they never touch the world directly. This eliminates race conditions on game state without a single mutex.

Each connected client runs two goroutines:
- **readPump** — reads lines from the TCP socket, forwards `Command` structs to the Hub
- **writePump** — drains a buffered `send` channel and writes responses/events to the socket

The Hub's `select {}` loop processes register/unregister/command events one at a time, calling the appropriate handler from a `verb → HandlerFunc` dispatch map.

---

## Protocol Implementation

> [!WARNING]
> **TODO (rusilva-)** — Document all deviations from RFC 42TAP with justification. Also document the scanner buffer size choice and max line length.

This implementation follows RFC 42TAP. Known deviations:

| Command | RFC / V.5 example | Our response | Reason |
|---------|-------------------|--------------|--------|
| `WHO`   | `OK players=<count>` | `OK {"room":[...],"server":<n>}` | Subject examples show richer format |
| `TALK`  | `OK <dialogue>` | `OK {"npc":"...","dialogue":"..."}` | Subject examples show JSON format |
| `LOOK` (items) | `"items":["item.herbs"]` | `"items":[{"id":"item.herbs","name":"Healing Herbs"}]` | GUI needs the display name to render the room without a separate lookup; avoids a fragile client-side ID→name cache |
| `LOOK` (npcs)  | `"npcs":["npc.baker"]` | `"npcs":[{"id":"npc.baker","name":"Baker","hostile":false}]` | Same as above, plus `hostile` flag so the GUI can colour enemies differently |
| `INVENTORY` | `OK ["item.herbs"]` | `OK [{"id":"item.herbs","name":"Healing Herbs"}]` | Same reasoning: GUI needs names to display inventory |

All deviations remain ABNF-compliant: the RFC's `response-data` production is `1*VCHAR`, so any JSON payload is valid. The V.5 examples in the subject are illustrative — the same examples already deviate from the RFC for `TALK` and `WHO`, confirming they are not literal specifications.

For interoperability with other groups' servers that follow the V.5 example formats, the GUI parses both shapes (plain ID string OR `{id, name}` object) inside `cmd/gui/main.go` (in the LOOK/INVENTORY response handlers) and normalises them before rendering.

---

## Combat System

> [!WARNING]
> **TODO (rusilva-)** — Document the damage formula, initiative order, and any extra commands implemented (DEFEND, FLEE). Fill in the placeholders below.

Design decisions:

- **Turn model**: each `ATTACK` command is one round; no persistent "in combat" state.
- **Damage formula**: `TODO — define and document here`
- **Counter-attack**: the NPC retaliates automatically each round unless defeated.
- **Defeat**: player HP reaches 0 → respawn at start room with 50% MaxHP.
- **Extra commands**: `TODO — DEFEND, FLEE (if implemented)`

---

## Quest System

> [!WARNING]
> **TODO (rusilva-)** — Document quest progression (NotStarted → Active → Completed), completion validation (auto vs manual), and reward distribution.

Supported quest types: `fetch` (bring item), `defeat` (kill NPC), `deliver` (give item to NPC).

State machine per player per quest: `NotStarted → Active → Completed`.

Completion is validated automatically when the player sends the relevant command (TAKE the target item, or after the NPC is defeated). Rewards are added directly to the player's inventory.

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

> [!WARNING]
> **TODO (rusilva-)** — Document the exact log format (JSON fields), log levels used, output stream, and the thresholds chosen for abuse detection.

Uses Go's `log/slog` with a JSON handler. All logs go to stdout.

| Event | Level | Fields |
|-------|-------|--------|
| Client connected | INFO | `ip`, `timestamp` |
| Client disconnected | INFO | `player`, `ip`, `reason` |
| Command received | INFO | `player`, `verb`, `args` |
| Response sent | INFO | `player`, `status`, `code` |
| Item moved | INFO | `item`, `from`, `to`, `player` |
| NPC interaction | INFO | `npc`, `player`, `type` |
| Combat result | INFO | `attacker`, `target`, `damage`, `result` |
| Quest event | INFO | `player`, `quest`, `event` |
| Abuse detected | WARN | `ip`, `player`, `pattern` |

> [!WARNING]
> **TODO (rusilva-)** — Define the rate-limit thresholds (commands per second, rapid reconnections) used to detect and log abuse patterns.

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