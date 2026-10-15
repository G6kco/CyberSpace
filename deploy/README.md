# Deploying CyberSpace

Production runs on `ps-server` (`192.168.20.241`, Ubuntu 24.04) in the lab
network, at `https://cyberspace.skysec.org`.

## How requests arrive

```
Outside the lab ─► Cloudflare ─► cloudflared ─► nginx 127.0.0.1:8090 ─┐
                                                                       ├─► API 127.0.0.1:8080
Lab PC (192.168.20.x) ───────────── LAN ─────► nginx 192.168.20.241:443 ┘
```

- The server is behind college NAT with no inbound ports, so the public site
  is served through a remotely managed Cloudflare Tunnel (`cloudflared`
  systemd service, token in `/etc/cloudflared/token`). Its public hostname
  `cyberspace.skysec.org` points at `http://localhost:8090`.
- Lab PCs resolve `cyberspace.skysec.org` to `192.168.20.241` (hosts file or
  lab DNS) and connect directly.
- `/api/v1/student/` (the student assessment API) is refused with
  `403 {"code":"lab_only"}` on the tunnel listener, and allowed on the LAN
  listener only from `192.168.20.0/24`. The client shows the message on every
  assessment page. Learning, sign-in and admin pages work everywhere.
- The restriction is by listener and source address, never by a request
  header, so it cannot be bypassed with `CF-Connecting-IP` or
  `X-Forwarded-For`. Any router that NATs other devices into the lab subnet
  makes them look like lab PCs; deny its address in `cyberspace.conf`.

## Layout on the server

| Path | Contents |
| --- | --- |
| `/opt/cyberspace/bin/` | `cyberspace-api`, `cyberspace-migrate`, `cyberspace-import-questions` |
| `/opt/cyberspace/web/` | built client (`Client/dist`) |
| `/opt/cyberspace/storage/{pcaps,files}` | captures and builder uploads (owned by `cyberspace`) |
| `/etc/cyberspace/api.env` | API environment (root:cyberspace, 640); generated on first install |
| `/etc/nginx/sites-available/cyberspace`, `/etc/nginx/snippets/cyberspace-*.conf` | from `deploy/nginx/` |
| `/etc/systemd/system/cyberspace-api.service` | from `deploy/systemd/` |

The API runs as the system user `cyberspace` (with the `docker` group for
labs), on MySQL database `cyberspace`.

## Releasing

Build on a development machine (the server's Go is too old for `go.mod`):

```bash
cd Server
for c in api migrate import-questions; do
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o ../release/bin/cyberspace-$c ./cmd/$c
done
cd ../Client
VITE_API_BASE_URL= npx vite build      # same origin: no API host in the bundle
cp -r dist ../release/web
cd ..
cp -r Server/storage/pcaps release/pcaps
cp -r deploy release/deploy
tar -czf cyberspace-release.tgz -C release .
```

Copy `cyberspace-release.tgz` and `deploy/server-setup.sh` to `/tmp` on the
server, then run `sudo bash /tmp/server-setup.sh`. It installs the binaries
and client, runs migrations, installs the nginx and systemd files, and
restarts both services.

## One-time steps

1. Cloudflare Zero Trust, Networks, Tunnels: the tunnel's public hostname
   `cyberspace.skysec.org` with service `http://localhost:8090`.
2. Google Cloud Console: authorised redirect URI
   `https://cyberspace.skysec.org/api/v1/auth/google/callback`.
3. LAN certificate, once the public hostname works:
   `sudo certbot certonly --webroot -w /var/www/letsencrypt -d cyberspace.skysec.org`,
   then re-run `server-setup.sh` (it switches from the self-signed
   certificate when the Let's Encrypt one exists) and add
   `--deploy-hook "systemctl reload nginx"` renewal via
   `/etc/letsencrypt/renewal-hooks/deploy/`.
4. Lab PCs: `192.168.20.241 cyberspace.skysec.org` in
   `C:\Windows\System32\drivers\etc\hosts`, or a lab DNS override.
5. Data: an `active` admin row in `users`, then
   `cyberspace-import-questions -dump cyber.sql -admin <admin email>`; seed
   `docker_hosts`, `network_pools` (name `macvlan`) and `network_addresses`
   with lab IPs that DHCP never hands out.

## Checking it

```bash
curl -s http://127.0.0.1:8080/health/ready                               # API
curl -s http://127.0.0.1:8090/api/v1/student/assessment                  # 403 lab_only
curl -sk --resolve cyberspace.skysec.org:443:192.168.20.241 \
  https://cyberspace.skysec.org/api/v1/student/assessment                # 401 from a lab address
journalctl -u cyberspace-api -f
```
