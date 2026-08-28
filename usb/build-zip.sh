#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN=""
WEB=""
OUT_ZIP="${HERE}/develop-tools-mundus.zip"

usage() {
  cat >&2 <<EOF
Usage: $(basename "$0") --bin PATH --web DIR [--out PATH]

  --bin PATH   the mundus server binary (linux/arm64)
  --web DIR    the built web UI directory (web/build)
  --out PATH   output zip path (default: ${OUT_ZIP})
EOF
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --bin) BIN="$2"; shift 2 ;;
    --web) WEB="$2"; shift 2 ;;
    --out) OUT_ZIP="$2"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "unknown arg: $1" >&2; usage 1 ;;
  esac
done

[ -n "$BIN" ] && [ -f "$BIN" ] || { echo "--bin (mundus arm64 binary) is required" >&2; exit 1; }
[ -n "$WEB" ] && [ -d "$WEB" ] || { echo "--web (built web dir) is required" >&2; exit 1; }
command -v zip >/dev/null || { echo "zip not found; install zip" >&2; exit 1; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
ROOT="$STAGE/develop_tools"
INSTALL="$ROOT/opt/wlab/sweepbot/mundus"
mkdir -p "$INSTALL/web" "$ROOT/etc/init.d" "$ROOT/etc/sudoers.d" \
  "$ROOT/etc/systemd/system/multi-user.target.wants"

cp "$BIN" "$INSTALL/mundus"
cp -R "$WEB/." "$INSTALL/web/"
printf 'mundus-installer\nbuilt=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)" > "$ROOT/develop_version"

cat > "$INSTALL/mundus.json" <<'EOJ'
{
  "device_id": "switchbot_s20",
  "device_name": "SwitchBot S20",
  "robot_addr": "127.0.0.1:50000",
  "mqtt_broker": "",
  "base_topic": "mundus",
  "discovery_prefix": "homeassistant",
  "web_addr": ":8080",
  "web_static": "/opt/wlab/sweepbot/mundus/web",
  "runtime_path": "/opt/wlab/sweepbot/mundus/runtime.json",
  "map_dir": "/data/map_server/map"
}
EOJ

# Boot hook: re-arm the developer guard (self-recovery) and start mundus as wlab.
# S99mundus sorts after the vendor's S99developer so it runs once the overlay is live.
cat > "$ROOT/etc/init.d/S99mundus" <<'EOS'
#!/bin/sh
# mundus: self-recovery re-arm + start the control server as wlab on boot.
DIR=/opt/wlab/sweepbot/mundus
case "$1" in
  start|"")
    rm -f /data/develop_version 2>/dev/null
    # The USB installer unzips as root, so everything under $DIR arrives
    # root-owned and mundus -- which runs as wlab -- cannot write its log,
    # runtime.json or auth.json there (the log redirect below fails outright).
    chown -R wlab:wlab "$DIR"
    if [ -x "$DIR/mundus" ]; then

      # Respawn loop with update rollback. An update points $DIR/mundus at the
      # other slot and leaves a marker naming the previous one; mundus removes
      # the marker once it is serving. If we get here with the marker still
      # present the new build never came up, so switch back. This is plain
      # shell on purpose -- it has to work when the new binary cannot exec.
      
      su wlab -c "setsid sh -c '
        while true; do
          \"$DIR/mundus\" -config \"$DIR/mundus.json\"
          if [ -f \"$DIR/mundus.update-pending\" ]; then
            slot=\$(cat \"$DIR/mundus.update-pending\")
            echo \"update did not come up; rolling back to \$slot\"
            ln -sfn \"mundus\$slot\" \"$DIR/mundus\"
            rm -f \"$DIR/mundus.update-pending\"
          fi
          sleep 5
        done' </dev/null >$DIR/mundus.log 2>&1 &"
    fi
    ;;
  stop)
    pkill -f "$DIR/mundus" 2>/dev/null
    ;;
esac
exit 0
EOS

# systemd is PID 1 here and there is no sysv-generator, so an init.d script on
# its own never runs. Every vendor SXX script has a unit that execs it; ours
# needs the same, plus the .wants entry that enables it.
cat > "$ROOT/etc/systemd/system/mundus.service" <<'EOU'
[Unit]
Description=mundus local control server
After=app.service network.target
Wants=network.target

[Service]
Type=oneshot
RemainAfterExit=yes
User=root
ExecStart=/etc/init.d/S99mundus

[Install]
WantedBy=multi-user.target
EOU

# Must be a symlink. A regular file here leaves the unit reachable by name but
# not wanted by multi-user.target, so it never starts at boot. zip -y below
# preserves it, and S99developer's cp -ar carries it into the overlay.
ln -sfn ../mundus.service \
  "$ROOT/etc/systemd/system/multi-user.target.wants/mundus.service"

printf 'wlab ALL=(ALL) NOPASSWD:ALL\n' > "$ROOT/etc/sudoers.d/wlab-nopasswd"

chmod 755 "$INSTALL/mundus" "$ROOT/etc/init.d/S99mundus"
chmod 644 "$INSTALL/mundus.json" "$ROOT/develop_version" \
  "$ROOT/etc/systemd/system/mundus.service"
chmod 440 "$ROOT/etc/sudoers.d/wlab-nopasswd"

rm -f "$OUT_ZIP"
mkdir -p "$(dirname "$OUT_ZIP")"
OUT_ZIP="$(cd "$(dirname "$OUT_ZIP")" && pwd)/$(basename "$OUT_ZIP")"
( cd "$STAGE" && zip -r -X -y -q "$OUT_ZIP" develop_tools )

echo ">> built $OUT_ZIP ($(du -h "$OUT_ZIP" | cut -f1))"
echo ">> installs mundus to /opt/wlab/sweepbot/mundus; SSH stays off (enable it in the web UI)."
