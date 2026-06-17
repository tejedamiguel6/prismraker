# Prismraker

**Per-color spool tracking + a live four-toolhead dashboard for the Snapmaker U1.**

The U1 is a four-toolhead color printer, but the stock Klipper/Moonraker/**Fluidd**
stack only supports single-extruder spool tracking — on a multi-color print it
decrements the *wrong* spool ([fluidd #1269](https://github.com/fluidd-core/fluidd/issues/1269),
[mainsail #1927](https://github.com/mainsail-crew/mainsail/issues/1927)).
Prismraker fixes exactly that, with a Go service + React dashboard that sit on top
of Moonraker through documented APIs — no forks.

> Submission for the [Snapmaker U1 Innovation Fund](https://forum.snapmaker.com/t/introducing-the-snapmaker-u1-innovation-fund/42210)
> open competition. MIT licensed.

## Repo layout

```
prismraker/
├── go-moonraker/                       # reusable Go client for Moonraker (WebSocket JSON-RPC)
│   └── moonraker/                       #   client.go (connect/call/subscribe), status.go (parse)
├── prismraker-svc/                      # the daemon
│   ├── cmd/prismraker-svc/
│   │   ├── main.go                      #   flags, wiring, applyStatus (Moonraker -> ledger)
│   │   ├── mock.go                      #   -mock simulated U1 (no hardware needed)
│   │   ├── capture.go                   #   -capture raw frames to JSONL for field validation
│   │   └── spools.go                    #   parseSpoolMap + demoSpools() catalog
│   └── internal/
│       ├── ledger/                      #   per-toolhead filament accounting (the multi-color fix)
│       ├── spoolman/                    #   Spoolman REST client (list/get/use)
│       ├── spoolsvc/                    #   runtime spool catalog + assignment + reconcile loop
│       ├── notify/                      #   color-aware event sinks (webhook/log)
│       └── api/                         #   REST + broadcast WebSocket to the UI
├── prismraker-ui/                       # React PWA: live 4-toolhead color dashboard
│   └── src/
│       ├── App.jsx                      #   layout + picker state + assign calls
│       ├── useToolheads.js              #   subscribes to /api/stream (falls back to demo data)
│       └── components/
│           ├── ToolheadCard.jsx         #   clickable card (a <button>)
│           └── SpoolPicker.jsx          #   modal to choose the loaded spool
└── docs/
```

`go-moonraker` is its own module so others can build on it — there is currently no
widely-used Go client for Moonraker.

## Quick start

**Try it with no printer (recommended first run):**

```bash
cd prismraker-svc && go run ./cmd/prismraker-svc -mock     # terminal 1, serves :8420
cd prismraker-ui  && npm install && npm run dev            # terminal 2, http://localhost:5173
```

`-mock` runs a simulated multi-color print (cycling toolheads, rising temps,
advancing filament) and preloads six demo spools, so the **whole UI including
click-to-assign works offline**.

**Against a real U1:**

```bash
cd prismraker-svc
go run ./cmd/prismraker-svc \
  -moonraker ws://YOUR-U1.local:7125/websocket \
  -spoolman http://localhost:7912          # optional: live spool colors/weights
# add -spoolman-sync to actually decrement spools (default is dry-run/log only)
```

### Useful flags (`prismraker-svc`)

| Flag | Default | Purpose |
|---|---|---|
| `-moonraker` | `ws://printer.local:7125/websocket` | U1 Moonraker websocket |
| `-mock` | `false` | Run without hardware using simulated data |
| `-toolheads` | `4` | Number of toolheads |
| `-spoolman` | `` | Spoolman base URL (enables live catalog) |
| `-spools` | `` | Optional startup assignment, e.g. `1,2,3,4` (T0→1…) |
| `-spoolman-sync` | `false` | Actually decrement spools (otherwise dry-run) |
| `-capture` | `` | Append raw Moonraker frames to a JSONL file for validation |
| `-listen` | `:8420` | API/UI server address |

## How it works

```
U1 ─ Klipper ─ Moonraker ──ws──► prismraker-svc ──► Spoolman (reconcile, dry-run by default)
                                      │            └► notify sinks (webhook/log)  [wired, not yet fired]
                                      └──ws/REST──► prismraker-ui (React)
```

The service subscribes to status, sets the **active** toolhead from
`toolhead.extruder`, and attributes deltas of the single `print_stats.filament_used`
counter to whichever tool is active — that's the multi-color accounting fix. Spool
colors/weights and the picker catalog come from Spoolman (or demo data offline).

### Interactive spool assignment (current focus)

The dashboard cards are clickable: click a toolhead → `SpoolPicker` lists the
catalog → choosing a spool assigns it. This replaced the old `-spools`-flag-only
workflow. Once a spool is loaded onto a toolhead, the reconcile loop decrements
**that** spool as its color is printed.

### HTTP/WS API (served by `internal/api`)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/toolheads` | Snapshot of all toolheads (index, name, colorHex, colorName, temp, usedMm, remainingGram, active) |
| `GET` | `/api/spools` | Catalog of assignable spools (`{id,name,colorHex,remainingGram}`) — live from Spoolman or demo |
| `POST` | `/api/assign` | Body `{toolhead:int, spoolId:int}`; `spoolId:0` clears. Triggers a broadcast |
| `GET` | `/api/stream` | WebSocket; pushes the toolheads snapshot on connect and on every change |
| `GET` | `/healthz` | Liveness |

## Status / notes for contributors

Works end-to-end in `-mock`; `go build ./...` + `go vet ./...` are green in both
`prismraker-svc` and `go-moonraker`, the React build is green, and the `ledger`
package has tests. **Still open:**

- **Live-U1 field names** (`toolhead.extruder`, `print_stats.filament_used`,
  `extruderN.*`) are based on a captured session; re-confirm with `-capture` against
  your machine, especially **usage attribution at tool-change boundaries** (purge/prime).
- **Notify sinks** exist but aren't fired from ledger events yet (low-spool/color-change).

Recently done: switched to the single-counter usage model and removed the dead
per-extruder `ObservePosition`/`lastPos` code and `Extruder.Position`/`PressureValue`;
added `ledger` tests covering attribution across a tool change.

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full plan.

## License

MIT — see [LICENSE](LICENSE). Built for the Snapmaker community.
