# hdg-exporter

Prometheus exporter for HDG Bavaria heating systems.

## Docker

A docker image is is available at `ghcr.io/srt/hdg-exporter`.
You can find a sample `docker-compose.yml` with Prometheus, Grafana and hdg-exporter in the `samples` subdirectory.
Just update the `HDG_ENDPOINT` environment variable.

## Configuration

HDG Exporter is configured via environment variables. The following variables are supported:

| Variable       | Description                                                                                                                                              | Example               |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| `HDG_ENDPOINT` | URL of the heating system.                                                                                                                               | `http://192.168.1.10` |
| `HDG_LANGUAGE` | One of `dansk`, `deutsch`, `english`, `franzoesisch`, `italienisch`, `niederlaendisch`, `norwegisch`, `polnisch`, `schwedisch`, `slowenisch`, `spanisch` | `deutsch`             |
| `HDG_WRITE_ENABLED` | Enable the write endpoints (`/api/set`, `/api/writable`). Off by default. Requires `HDG_WRITE_TOKEN` and a non-empty `HDG_WRITE_ALLOW`.                       | `true`                |
| `HDG_WRITE_TOKEN` | Bearer token required for `POST /api/set`. Keep it secret; serve the exporter only on a trusted network.                                                   |                       |
| `HDG_WRITE_ALLOW` | JSON allowlist of writable parameters (see below). Only listed ids can be written; values outside the bounds are rejected before any request reaches the HDG. | *(see below)*         |
| `HDG_TIMEZONE` | Time zone of the heating system's clock, used for the logbook timestamps. Default `Europe/Berlin`.                                                       | `Europe/Berlin`       |
| `HDG_IDS`      | Comma separated list of ids to export. Can be obtained from the Web UI or from [data.json](data.json).                                                   |                       |

Formatter XML files are embedded into the exporter binary at build time. Place
the files in [formatters](formatters) using the name
`<HDG_LANGUAGE>_formatters.xml`, for example `deutsch_formatters.xml`, before
running `go build` or building the Docker image.

## Logbooks

Besides `/metrics`, the exporter serves the WebControl's logbooks as JSON. The WebControl only
renders them as HTML; the exporter fetches and parses the page on each request.

- `GET /api/messages`: the message logbook (faults `ER…` and warnings `WA…`), newest first.
- `GET /api/maintenance`: the maintenance logbook (acknowledged maintenance tasks), newest first.

```json
[{"timestamp":"2026-11-09T11:12:00+01:00","code":"ER037","kind":"fault","text":"Störung Reinigung","state":"on"}]
```

`kind` is `fault`, `warning` or `other`; `state` is `on` (coming) or `off` (going).

```json
[{"timestamp":"2026-11-16T12:02:00+01:00","task":"Aschenbehälter leeren (Störung)"}]
```

The heating system keeps only a limited number of entries, so poll and store them if you need the
full history. Texts are in the WebControl's language. The overall message status is available as
metric `hdg_value{id="20029"}` (0 = none, 1 = warning, 2 = fault).

## Writing values (optional, off by default)

With `HDG_WRITE_ENABLED=true`, a token and an allowlist, the exporter can set parameters on the
heating system. This is **disabled by default** — writing wrong parameters on a heating system is
risky, so only the ids you explicitly list can ever be written, and the exporter validates every
value before it reaches the HDG.

`HDG_WRITE_ALLOW` is a JSON array; each entry is `number` (with optional `min`/`max`/`step`/`unit`)
or `enum` (with the allowed `keys`):

```json
[
  {"id": 6022,  "type": "number", "min": 15, "max": 24, "step": 0.5, "unit": "°C"},
  {"id": 20032, "type": "enum",   "keys": ["NORMAL", "URLAUB", "SOMMER"]}
]
```

- `GET /api/writable` — the allowlist, so a client can render controls.
- `POST /api/set` (requires `Authorization: Bearer <HDG_WRITE_TOKEN>`): body
  `{"id": 6022, "value": 21.5}` for a number or `{"id": 20032, "key": "URLAUB"}` for an enum.
  The exporter writes the change, then polls the read-back and answers
  `{"id":…, "status":"applied|pending|rejected", "requested":…, "confirmed":…, "error":…}`.
  `applied` = the heating system now reports the value; `pending` = accepted but not yet reflected
  (the HDG applies changes asynchronously); `rejected` = the value was refused by the allowlist.

The heating system itself does **not** check the id against any allowlist and (on the firmware
tested) does not authenticate `set_value_changed` — the exporter is the only guard, so keep the
allowlist minimal and the exporter on a trusted network.

## Grafana Dashboard

![Grafana Dashboard](grafana/dashboard.png)

The [Dashboard](sample/grafana/provisioning/dashboards/HDG.json) can be imported into Grafana.

## Similar Projects

- [hdg-bavaria adapter for ioBroker](https://github.com/SteMaker/ioBroker.hdg-bavaria) includes a [wiki](https://github.com/SteMaker/ioBroker.hdg-bavaria/wiki) with a howto to lookup id values
