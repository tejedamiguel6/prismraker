# Roadmap

Aligned to the Innovation Fund Phase 1 window (Jun 9 – Sep 7, 2026).

## Validate first (before trusting accounting)

- [ ] Run `Snapmaker/u1-moonraker` locally; capture a real `notify_status_update`
      frame and confirm the per-extruder object/field names (`extruder`, `extruder1`…).
- [ ] Identify the **active-tool** signal in the U1 fork (toolhead/tool object) and
      wire `ledger.SetActive`.
- [ ] Confirm the cumulative filament-position field used for usage deltas.
- [ ] Confirm Spoolman REST endpoints for setting/decrementing a specific spool.

## Milestones

- [x] M1 — `go-moonraker`: connect + subscribe + parse status updates
- [x] M1 — `prismraker-svc`: ledger, notify, API scaffold; runnable
- [x] M1 — `prismraker-ui`: live 4-toolhead dashboard (demo + ws)
- [ ] M2 — correct per-toolhead usage attribution across tool changes
- [ ] M3 — Spoolman reconciliation (right spool decrements)
- [ ] M4 — color-aware notifications (ntfy/Discord/webhook) fired from ledger events
- [ ] M5 — swatch assignment UI + Spoolman color metadata
- [ ] M6 — mobile PWA polish, install docs, demo video, submit

## Stretch

- [ ] Multi-printer fleet view (several U1s in one dashboard)
- [ ] Reconnect-with-backoff in `go-moonraker`
- [ ] Home Assistant friendly event feed
