# vpngate-csv-hostless

Minimal authenticated VPNGate CSV proxy, deployed on Hostless.
Contract mirrors the Cloudflare Worker `vpngate-csv-proxy`:

- `GET /csv` with `Authorization: Bearer <API_TOKEN>` (or `X-API-Key`) returns the
  live VPNGate CSV from `https://www.vpngate.net/api/iphone/`, cached in memory for 120s.
- `GET /health` returns `{"status":"ok"}` (platform health check, no auth).
- Everything else returns 404. If `API_TOKEN` is not configured, `/csv` returns 503.

`API_TOKEN` is set as an environment variable in the Hostless console only —
never commit it to this repository.
