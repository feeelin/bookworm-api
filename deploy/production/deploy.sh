#!/bin/sh
# Blue-green deployment. Usage: sh deploy.sh <full image path with tag>
set -eu

IMAGE="$1"
VERSION="${IMAGE##*:}"
UPSTREAM=/etc/nginx/conf.d/upstream.conf

log() {
  echo "[deploy] $*"
}

cd /srv/production

# 1. Detect which color is currently active
if docker compose exec -T nginx grep -q "app-blue" "$UPSTREAM"; then
  ACTIVE=blue
  NEW=green
else
  ACTIVE=green
  NEW=blue
fi
log "Active color: $ACTIVE"
log "Deploying version $VERSION to: $NEW"

# 2. Set the new image for the inactive color
VAR="APP_IMAGE_$(echo "$NEW" | tr a-z A-Z)"
sed -i "s|^${VAR}=.*|${VAR}=${IMAGE}|" .env
log "Updated $VAR in .env"

# 3. Start the inactive color, leave the active one untouched
log "Pulling image for app-$NEW"
docker compose pull "app-$NEW"
log "Starting app-$NEW"
docker compose up -d --no-deps --force-recreate "app-$NEW"

# 4. Wait until the health check turns green
log "Waiting for app-$NEW to become healthy (up to 60 seconds)"
CID=$(docker compose ps -q "app-$NEW")
STATUS=starting
i=0
while [ "$i" -lt 30 ]; do
  STATUS=$(docker inspect -f '{{.State.Health.Status}}' "$CID")
  [ "$STATUS" = "healthy" ] && break
  i=$((i + 1))
  sleep 2
done
if [ "$STATUS" != "healthy" ]; then
  log "ERROR: app-$NEW did not become healthy (status: $STATUS), traffic was NOT switched"
  docker compose logs --tail 50 "app-$NEW"
  exit 1
fi
log "app-$NEW is healthy"

# 5. Check the version on the new color while users are still on the old one
ANSWER=$(docker compose exec -T nginx wget -qO- "http://app-$NEW:8080/version")
log "app-$NEW reports: $ANSWER"
case "$ANSWER" in
  *"$VERSION"*) log "Version check passed" ;;
  *)
    log "ERROR: expected version $VERSION, traffic was NOT switched"
    exit 1
    ;;
esac

# 6. Switch traffic
log "Switching traffic to $NEW"
docker compose exec -T nginx cp "$UPSTREAM" "$UPSTREAM.bak"
docker compose exec -T nginx sh -c "printf 'upstream app {\n    server app-$NEW:8080;\n}\n' > $UPSTREAM"
if ! docker compose exec -T nginx nginx -t; then
  docker compose exec -T nginx cp "$UPSTREAM.bak" "$UPSTREAM"
  log "ERROR: nginx config test failed, previous config restored, traffic was NOT switched"
  exit 1
fi
docker compose exec -T nginx nginx -s reload

log "Done: traffic is now on $NEW ($VERSION), $ACTIVE is kept running for rollback"