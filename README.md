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
├── go-moonraker/      # reusable Go client for Moonraker (WebSocket JSON-RPC)
├── prismraker-svc/    # the daemon: per-toolhead ledger + Spoolman + notifications + API
├── prismraker-ui/     # React PWA: live 4-toolhead color dashboard
└── docs/
```

`go-moonraker` is published as its own module so others can build on it — there is
currently no widely-used Go client for Moonraker.

## Quick start

**Service (Go 1.22+):**

```bash
cd prismraker-svc
go run ./cmd/prismraker-svc -moonraker ws://YOUR-U1.local:7125/websocket
# serves the API on :8420
```

**Dashboard (Node 18+):**

```bash
cd prismraker-ui
npm install
npm run dev      # http://localhost:5173, proxies /api to :8420
```

The UI ships with demo data, so it renders even before the service is connected to
a real printer.

## How it works

```
U1 ─ Klipper ─ Moonraker ──ws──► prismraker-svc ──► Spoolman (reconcile)
                                      │            └► Discord/ntfy/webhook (alerts)
                                      └──ws/REST──► prismraker-ui (React)
```

The service subscribes to per-extruder status, attributes filament usage to the
**active** toolhead across color changes, and reconciles against Spoolman so the
right spool decrements.

## Status

Early scaffold — connects, subscribes, tracks, and serves. The `TODO`s in
`prismraker-svc` mark fields that must be confirmed against the open-sourced U1 fork
([Snapmaker/u1-moonraker](https://github.com/Snapmaker/u1-moonraker)) on a live
machine before the per-color accounting is trusted. See [docs/ROADMAP.md](docs/ROADMAP.md).

## License

MIT — see [LICENSE](LICENSE). Built for the Snapmaker community.
