#!/bin/sh
# Blue-green rollback: switch traffic back to the inactive color.
# Usage: sh rollback.sh
set -eu

UPSTREAM=/etc/nginx/conf.d/upstream.conf

log() {
  echo "[rollback] $*"
}

cd /srv/production

# 1. Detect the active color and the rollback target
if docker compose exec -T nginx grep -q "app-blue" "$UPSTREAM"; then
  ACTIVE=blue
  TARGET=green
else
  ACTIVE=green
  TARGET=blue
fi
log "Active color: $ACTIVE"
log "Rollback target: $TARGET"

# 2. Make sure the target is alive before sending users to it
if ! docker compose exec -T nginx wget -qO- "http://app-$TARGET:8080/health" >/dev/null 2>&1; then
  log "ERROR: app-$TARGET does not answer on /health, rollback by switching is impossible"
  log "Redeploy the previous release tag instead"
  exit 1
fi
log "app-$TARGET is healthy"

# 3. Show what we are rolling back from and to
ACTIVE_VERSION=$(docker compose exec -T nginx wget -qO- "http://app-$ACTIVE:8080/version" 2>/dev/null || echo "unknown")
TARGET_VERSION=$(docker compose exec -T nginx wget -qO- "http://app-$TARGET:8080/version" 2>/dev/null || echo "unknown")
log "Current ($ACTIVE): $ACTIVE_VERSION"
log "Rolling back to ($TARGET): $TARGET_VERSION"

# 4. Switch traffic
log "Switching traffic to $TARGET"
docker compose exec -T nginx cp "$UPSTREAM" "$UPSTREAM.bak"
docker compose exec -T nginx sh -c "printf 'upstream app {\n    server app-$TARGET:8080;\n}\n' > $UPSTREAM"
if ! docker compose exec -T nginx nginx -t; then
  docker compose exec -T nginx cp "$UPSTREAM.bak" "$UPSTREAM"
  log "ERROR: nginx config test failed, previous config restored, traffic was NOT switched"
  exit 1
fi
docker compose exec -T nginx nginx -s reload

# 5. Verify through the public entrypoint
RESULT=$(docker compose exec -T nginx wget -qO- http://localhost/version)
log "nginx now serves: $RESULT"

log "Done: traffic is now on $TARGET, $ACTIVE is kept running for investigation"