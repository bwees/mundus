#!/usr/bin/env bash
# Push WiFi credentials to a SwitchBot S20 over BLE, from a Linux host with BlueZ.
#
# Protocol: ../../switchbot-research/docs/ble-wifi-provisioning.md, plus the
# credential layout recovered from control_center_runner and written up in
# tools/README.md. The method byte selects the field; the payload is the raw
# UTF-8 value with no prefix.
set -euo pipefail

SERVICE_UUID=cba20d00-224d-11e6-9fb8-0002a5d5c51b
WRITE_UUID=cba20002-224d-11e6-9fb8-0002a5d5c51b
NOTIFY_UUID=cba20003-224d-11e6-9fb8-0002a5d5c51b

M_HELLO=0x01
M_STATUS=0x07
M_SCAN=0x08
M_COUNT=0x09
M_LIST=0x0A

# combinedNetworkData() switches on the method byte to pick the field it fills.
M_SSID=0x05
M_PASSWD=0x06
# 0x0C stores is_need_bind=false ("only set wifi"); its payload is ignored.
M_WIFI_ONLY=0x0C

MAC=""
SSID=""
PASSWORD=""
SCAN_ONLY=0
DRY_RUN=0
SCAN_SECS=12
LOG=""

usage() {
  cat >&2 <<EOF
Usage: $(basename "$0") [--ssid NAME --password PASS] [options]

  --ssid NAME        WiFi network to join
  --password PASS    WiFi password (omit for an open network)
  --mac AA:BB:..     skip discovery and use this address
  --scan-only        run only the confirmed read-only exchange and dump replies
  --dry-run          print the frames that would be sent, touch no hardware
  --scan-secs N      how long to scan for the robot (default ${SCAN_SECS})
  --log FILE         also write the raw bluetoothctl transcript here
  -h, --help         this message

Reading the password from the environment avoids putting it in your shell
history: S20_WIFI_PASSWORD=hunter2 $(basename "$0") --ssid home
EOF
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --ssid) SSID="$2"; shift 2 ;;
    --password) PASSWORD="$2"; shift 2 ;;
    --mac) MAC="$2"; shift 2 ;;
    --scan-only) SCAN_ONLY=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --scan-secs) SCAN_SECS="$2"; shift 2 ;;
    --log) LOG="$2"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "unknown argument: $1" >&2; usage 1 ;;
  esac
done

[ -n "$PASSWORD" ] || PASSWORD="${S20_WIFI_PASSWORD:-}"

if [ "$SCAN_ONLY" -eq 0 ] && [ -z "$SSID" ]; then
  echo "--ssid is required (or use --scan-only)" >&2
  usage 1
fi

# bytes_of emits the UTF-8 bytes of a string as 0xNN tokens. od -tx1 rather than
# a character loop so a non-ASCII SSID survives byte-for-byte.
bytes_of() {
  [ -n "$1" ] || return 0
  printf '%s' "$1" | od -An -tx1 -v | tr -s ' ' '\n' | sed '/^$/d;s/^/0x/' | tr '\n' ' '
}

# frame builds 5A <method> <total> <index> <len> <payload...> <xor>, where xor
# folds every byte before it. The robot concatenates payloads until it has
# <total> packets, so a value that fits in one frame sends total=index=1.
frame() {
  local method="$1"; shift
  local -a payload=("$@")
  local -a bytes=(0x5A "$method" 0x01 0x01 "$(printf '0x%02x' "${#payload[@]}")")
  [ "${#payload[@]}" -gt 0 ] && bytes+=("${payload[@]}")
  local xor=0 b
  for b in "${bytes[@]}"; do xor=$(( xor ^ b )); done
  bytes+=("$(printf '0x%02x' "$xor")")
  printf '%s' "${bytes[*]}"
}

need() {
  command -v "$1" >/dev/null || { echo "missing $1. $2" >&2; exit 1; }
}

discover() {
  echo ">> scanning ${SCAN_SECS}s for a SwitchBot robot..." >&2
  local out
  out="$( { echo "scan on"; sleep "$SCAN_SECS"; echo "scan off"; echo "exit"; } | bluetoothctl 2>/dev/null || true )"

  local mac
  mac="$(printf '%s\n' "$out" | grep -iE 'Device ([0-9A-F:]{17}) WoS' | grep -oE '([0-9A-F]{2}:){5}[0-9A-F]{2}' | head -1 || true)"
  if [ -z "$mac" ]; then
    echo "no robot found." >&2
    echo "The S20 only advertises its name while in pairing mode. Hold the" >&2
    echo "robot's reset/WiFi button until it announces network pairing, then retry." >&2
    echo "If you know the address, pass --mac." >&2
    exit 1
  fi
  printf '%s' "$mac"
}

FRAME_HELLO="$(frame "$M_HELLO")"
FRAME_SCAN="$(frame "$M_SCAN")"
FRAME_COUNT="$(frame "$M_COUNT")"
FRAME_LIST="$(frame "$M_LIST")"
FRAME_STATUS="$(frame "$M_STATUS")"
FRAME_SSID="$(frame "$M_SSID" $(bytes_of "$SSID"))"
FRAME_WIFI_ONLY="$(frame "$M_WIFI_ONLY")"
FRAME_PASSWD=""
[ -n "$PASSWORD" ] && FRAME_PASSWD="$(frame "$M_PASSWD" $(bytes_of "$PASSWORD"))"

if [ "$DRY_RUN" -eq 1 ]; then
  echo "hello    $FRAME_HELLO"
  echo "scan     $FRAME_SCAN"
  echo "count    $FRAME_COUNT"
  echo "list     $FRAME_LIST"
  if [ "$SCAN_ONLY" -eq 0 ]; then
    echo "ssid     $FRAME_SSID"
    if [ -n "$FRAME_PASSWD" ]; then
      echo "passwd   $(printf '%s' "$FRAME_PASSWD" | sed -E 's/(0x5A 0x06 0x01 0x01 0x[0-9a-f]{2}).*( 0x[0-9a-f]{2})$/\1 <password bytes>\2/')"
    fi
    echo "wifionly $FRAME_WIFI_ONLY"
  fi
  echo "status   $FRAME_STATUS"
  exit 0
fi

need bluetoothctl "Install BlueZ (apt install bluez)."
need od "coreutils is required."

if ! bluetoothctl show 2>/dev/null | grep -q "Powered: yes"; then
  echo "no powered Bluetooth adapter. Try: bluetoothctl power on" >&2
  exit 1
fi

[ -n "$MAC" ] || MAC="$(discover)"
echo ">> robot at $MAC" >&2

# bluetoothctl reads commands from stdin, so the session is a timed script. The
# sleeps are the synchronisation: there is no request/response handshake to wait on.
session() {
  echo "connect $MAC"
  sleep 6
  echo "menu gatt"
  echo "list-attributes $MAC"
  sleep 2
  echo "select-attribute $NOTIFY_UUID"
  echo "notify on"
  sleep 2
  echo "select-attribute $WRITE_UUID"

  echo "write \"$FRAME_HELLO\""
  sleep 1
  echo "write \"$FRAME_SCAN\""
  sleep 6
  echo "write \"$FRAME_COUNT\""
  sleep 2
  echo "write \"$FRAME_LIST\""
  sleep 3

  if [ "$SCAN_ONLY" -eq 0 ]; then
    echo "write \"$FRAME_SSID\""
    sleep 2
    if [ -n "$FRAME_PASSWD" ]; then
      echo "write \"$FRAME_PASSWD\""
      sleep 2
    fi
    echo "write \"$FRAME_WIFI_ONLY\""
    sleep 4
    echo "write \"$FRAME_STATUS\""
    sleep 5
  fi

  echo "back"
  echo "disconnect $MAC"
  sleep 1
  echo "exit"
}

transcript="$(mktemp)"
trap 'rm -f "$transcript"' EXIT

session | bluetoothctl 2>&1 | tee "$transcript" | grep --line-buffered -E 'Notification|Value:|Connected|Attempting|Failed|error' || true

[ -n "$LOG" ] && cp "$transcript" "$LOG" && echo ">> transcript written to $LOG" >&2

if ! grep -qi "$WRITE_UUID" "$transcript"; then
  echo >&2
  echo "!! the robot never listed $WRITE_UUID." >&2
  echo "   The provisioning characteristic UUIDs are inferred, not air-confirmed." >&2
  echo "   Re-run with --log and send the attribute list so they can be corrected." >&2
  exit 1
fi

echo >&2
if [ "$SCAN_ONLY" -eq 1 ]; then
  echo ">> read-only exchange finished. Any 'Value:' lines above are the robot's" >&2
  echo "   replies to the confirmed scan opcodes, which proves the pipe works." >&2
else
  echo ">> credentials sent, then is_need_bind=false so the robot joins WiFi without" >&2
  echo "   binding to a SwitchBot account. Give it 10-30s and check your DHCP leases." >&2
  echo "   If it does not appear, re-run with --log and see tools/README.md." >&2
fi
