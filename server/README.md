# mundus server

The Go control server that runs on the robot. It drives the vendor's
`control_center`, bridges the vacuum to Home Assistant over MQTT, and serves the
web UI and its JSON API on `:8080`.

This README is for developers. For installation instructions, see
[usb/README.md](usb/README.md) for instructions.

## Two ways into control_center

Commands go through the local **funcRequest HTTP API**. `control_center`
generates a random port and token at runtime, and mundus reads both back over the
terminal. Sending a high-level funcID lets `control_center` run its own
choreography for wake, relocate, clean, servicing and docking, so cleaning
behaves the way the stock firmware intends.

Telemetry and four base actions go through the **cpp-tbox text terminal** on
`127.0.0.1:50000`. mundus polls it for state, reads room labels, and runs the
handful of commands with no funcRequest equivalent.

`internal/robot/commands.go` tags each terminal command with how far it was
verified on real hardware. `[C]` is confirmed on a live S20, `[A]` means the node
exists but its arguments are not.

## Build

```sh
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o mundus .
```

From the repo root, `mise run build` cross-compiles and `mise run dist` adds the
web bundle.

## Configure

Copy `mundus.example.json` and pass it with `-config`. Every path in it defaults
to a stock install, so a minimal config sets nothing at all and you configure the
rest from the web UI.

Other flags: `-openapi` writes `openapi.json` and exits, `-rollback` undoes an
unconfirmed update and is only for the boot loop.

## Layout

```
internal/config     JSON and env config, runtime MQTT overlay
internal/auth       bcrypt admin credential
internal/funcapi    funcRequest transport, token acquire and refresh
internal/robotapi   typed funcID calls, map files, property cache
internal/robot      terminal client, telemetry parsing, command table
internal/settings   the one table of device settings, shared by hass and webapi
internal/hass       MQTT discovery and bridge loop
internal/mapgeo     rasterise, union, split, contour room polygons
internal/webapi     JSON API, auth guard, static file serving
internal/update     A/B slots, release download, checksum verify
```

Every `/api/*` route needs a bearer token except `/api/auth/status`, `/api/auth/setup`
and `/api/auth/login`. `internal/webapi/auth_test.go` walks `openapi.json` and
fails if any route answers without one. A route hidden with `OptionHide()` has to
be added to that test by hand.

