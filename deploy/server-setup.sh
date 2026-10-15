#!/usr/bin/env bash
# Installs or updates CyberSpace on ps-server. Idempotent: safe to re-run.
# See deploy/README.md for the release archive it expects in /tmp.
#
# First install only (while /etc/cyberspace/api.env does not exist): it
# creates the database and env file, reading the MySQL root password from the
# old portal's env file and the Google settings from /tmp/google.env.
set -euo pipefail

REL=/tmp/cyberspace-release
rm -rf "$REL"; mkdir -p "$REL"
tar -xzf /tmp/cyberspace-release.tgz -C "$REL"

echo "== packages"
if ! command -v nginx >/dev/null || ! command -v certbot >/dev/null; then
  apt-get update -qq
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nginx certbot ssl-cert >/dev/null
fi
rm -f /etc/nginx/sites-enabled/default

echo "== user and directories"
id cyberspace >/dev/null 2>&1 || useradd --system --home-dir /opt/cyberspace --shell /usr/sbin/nologin cyberspace
install -d -m 755 /opt/cyberspace /opt/cyberspace/bin /var/www/letsencrypt
install -d -m 750 -o cyberspace -g cyberspace /opt/cyberspace/storage /opt/cyberspace/storage/pcaps /opt/cyberspace/storage/files
install -m 755 "$REL"/bin/* /opt/cyberspace/bin/
rm -rf /opt/cyberspace/web.new; cp -r "$REL/web" /opt/cyberspace/web.new; chmod -R a+rX /opt/cyberspace/web.new
rm -rf /opt/cyberspace/web; mv /opt/cyberspace/web.new /opt/cyberspace/web
cp "$REL"/pcaps/* /opt/cyberspace/storage/pcaps/
chown -R cyberspace:cyberspace /opt/cyberspace/storage

echo "== database and env"
if [ ! -f /etc/cyberspace/api.env ]; then
  ROOTPW=$(grep '^DB_PASS=' /home/ps/backend/server/.env | cut -d= -f2- | tr -d '"'"'"'\r')
  # validate_password wants upper and lower case, digits and a symbol.
  DBPW="Cs_$(openssl rand -hex 24)X"
  MYSQL_PWD="$ROOTPW" mysql -uroot -e "
    CREATE DATABASE IF NOT EXISTS cyberspace CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
    CREATE USER IF NOT EXISTS 'cyberspace'@'localhost' IDENTIFIED BY '$DBPW';
    ALTER USER 'cyberspace'@'localhost' IDENTIFIED BY '$DBPW';
    GRANT ALL PRIVILEGES ON cyberspace.* TO 'cyberspace'@'localhost';"
  install -d -m 750 -o root -g cyberspace /etc/cyberspace
  umask 027
  {
    echo 'APP_ENV=production'
    echo 'SERVER_HOST=127.0.0.1'
    echo 'SERVER_PORT=8080'
    echo "DATABASE_URL=\"cyberspace:$DBPW@tcp(127.0.0.1:3306)/cyberspace?multiStatements=true&parseTime=true\""
    echo 'FRONTEND_URL=https://cyberspace.skysec.org'
    echo 'CORS_ORIGIN_ALLOWED=https://cyberspace.skysec.org'
    echo 'GOOGLE_CALL_BACK=https://cyberspace.skysec.org/api/v1/auth/google/callback'
    echo "SESSION_SECRET=$(openssl rand -hex 32)"
    echo "ANSWER_HMAC_KEY=$(openssl rand -hex 32)"
    echo 'PCAP_DIR=/opt/cyberspace/storage/pcaps'
    echo 'FILES_DIR=/opt/cyberspace/storage/files'
    tr -d '\r' < /tmp/google.env
  } > /etc/cyberspace/api.env
  chown root:cyberspace /etc/cyberspace/api.env; chmod 640 /etc/cyberspace/api.env
fi
rm -f /tmp/google.env

echo "== migrations"
runuser -u cyberspace -- bash -c 'set -a; . /etc/cyberspace/api.env; set +a; cd /opt/cyberspace; ./bin/cyberspace-migrate up'

echo "== nginx"
install -m 644 "$REL"/deploy/nginx/cyberspace-common.conf "$REL"/deploy/nginx/cyberspace-proxy.conf /etc/nginx/snippets/
install -m 644 "$REL"/deploy/nginx/cyberspace.conf /etc/nginx/sites-available/cyberspace
if [ ! -f /etc/letsencrypt/live/cyberspace.skysec.org/fullchain.pem ]; then
  # Until Let's Encrypt issues the real certificate, the LAN listener uses
  # the self-signed one so nginx can start.
  sed -i 's#/etc/letsencrypt/live/cyberspace.skysec.org/fullchain.pem#/etc/ssl/certs/ssl-cert-snakeoil.pem#; s#/etc/letsencrypt/live/cyberspace.skysec.org/privkey.pem#/etc/ssl/private/ssl-cert-snakeoil.key#' /etc/nginx/sites-available/cyberspace
fi
ln -sf /etc/nginx/sites-available/cyberspace /etc/nginx/sites-enabled/cyberspace
nginx -t
systemctl enable --now nginx >/dev/null 2>&1
# Restart, not reload: the stock site's 0.0.0.0:80 socket blocks new binds.
systemctl restart nginx

echo "== api service"
install -m 644 "$REL"/deploy/systemd/cyberspace-api.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable cyberspace-api >/dev/null 2>&1
systemctl restart cyberspace-api
sleep 4
systemctl is-active cyberspace-api
echo "== done"
