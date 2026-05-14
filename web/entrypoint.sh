#!/bin/sh
# =============================================================================
# KeyIP Web — Nginx entrypoint
# =============================================================================
# Reads NGINX_USE_STUBS env var. When set to "true", copies nginx-stubs.conf
# into the include path so nginx serves hardcoded mock data for all API
# endpoints. When false/unset, /etc/nginx/stubs.conf is kept as an empty file,
# and all API requests proxy through to the apiserver.
# =============================================================================

STUBS_PATH="/etc/nginx/stubs.conf"
STUBS_SRC="/usr/share/nginx/html/nginx-stubs.conf"

# Always start with an empty stub file (nginx includes it unconditionally)
echo -n > "$STUBS_PATH"

if [ "${NGINX_USE_STUBS}" = "true" ]; then
    if [ -f "$STUBS_SRC" ]; then
        echo "[entrypoint] NGINX_USE_STUBS=true — enabling API stubs"
        cp "$STUBS_SRC" "$STUBS_PATH"
    else
        echo "[entrypoint] WARNING: NGINX_USE_STUBS=true but $STUBS_SRC not found"
    fi
else
    echo "[entrypoint] NGINX_USE_STUBS not set — API requests will proxy to apiserver"
fi

exec nginx -g 'daemon off;'
