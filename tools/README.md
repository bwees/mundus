# tools

Host-side utilities. These run on your computer, not on the robot.

## s20-wifi-setup.sh

Pushes WiFi credentials to a SwitchBot S20 over BLE, which is how you get a
factory-reset or re-homed robot onto the network without the SwitchBot app.

```sh
# put the robot in network-pairing mode first, then:
S20_WIFI_PASSWORD='hunter2' ./tools/s20-wifi-setup.sh --ssid 'HomeNet'

./tools/s20-wifi-setup.sh --scan-only     # confirmed read-only exchange
./tools/s20-wifi-setup.sh --dry-run --ssid X   # print frames, touch no hardware
```

Needs Linux with BlueZ (`bluetoothctl`). macOS has no scriptable GATT client, so
there is no shell version there. The robot only advertises its name in pairing
mode, so hold the reset/WiFi button until it announces network pairing.

Nothing here is encrypted or authenticated. The provisioning path takes plaintext
frames with a one-byte XOR checksum, so anyone in radio range of a robot in
pairing mode can set its WiFi. That is the vendor's design, not ours.

## The protocol

Frames go to the SwitchBot GATT pipe, service `cba20d00-224d-11e6-9fb8-0002a5d5c51b`,
write `cba20002-…`, notify `cba20003-…`.

```
5A | method | total | index | len | payload… | xor
```

`xor` folds every preceding byte. `total` is how many frames make up this value
and `index` is the position within it. The robot concatenates payloads until it
has `total` frames, then acts. A value short enough for one frame sends
`total=1, index=1`, which covers any SSID (≤32 B) and any PSK (≤63 B).

**The method byte selects the field.** There is no selector or type byte inside
the payload; the payload is the raw UTF-8 value.

| method | field | payload |
|---:|---|---|
| `0x01` | handshake | empty |
| `0x03` | `https` | URL string |
| `0x04` | `token` | cloud binding token |
| `0x05` | **`ssid`** | network name |
| `0x06` | **`passwd`** | network password |
| `0x07` | status query | empty |
| `0x08` | start AP scan | empty |
| `0x09` | scanned AP count | empty |
| `0x0A` | scanned AP list | empty |
| `0x0B` | `utc_time` | time string |
| `0x0C` | `is_need_bind` | ignored, always stores `false` |

Send `0x05`, then `0x06`, then `0x0C`. That last one is what makes the robot join
the network and stop there instead of also binding to a SwitchBot account.

## How the field table was recovered

`switchbot-research/docs/ble-wifi-provisioning.md` had the framing, the checksum
and the scan opcodes confirmed from disassembly, but marked the credential layout
`[open]`: it guessed a one-byte selector inside the payload, with `0x0A` carrying
the SSID. That guess was wrong in both respects. The real answer is in
`control_center_runner` (aarch64, stripped of `.symtab` but with a full
`.dynsym`), reachable with `objdump` alone:

```sh
cd switchbot-research/analysis/binaries
objdump -T control_center_runner | grep combinedNetworkData
objdump -d --start-address=0x38bd58 --stop-address=0x38c788 control_center_runner
```

`BtSlaveInterface::combinedNetworkData(const uint8&, const std::string&)` at
`0x38bd58` switches its first argument against `3, 4, 5, 6, 0x0B, 0x0C`. Each arm
builds a JSON field, and resolving the `adrp`/`add` pair in each arm gives the
name it writes: `https`, `token`, `ssid`, `passwd`, `utc_time`, `is_need_bind`.

Its only caller is `parseReportMethodData` at `0x38c788`, which runs `parsePacket`
and then `spliceData`. `spliceData`'s implementation at `0x388c50` shows where the
key comes from:

```
388d28: ldrb w0, [x19, #0x1]   ; DataPacket+1 = total packet count
388d3c: cmp  x1, x0            ; collected == total ?
...
388dc8: ldrb w0, [x19]         ; DataPacket+0 = the METHOD byte
388dcc: strb w0, [x22]         ; -> the uint8& key handed to combinedNetworkData
```

So the key is the frame's method byte, and `DataPacket+1` (which the spec read as
a constant `0x01` flag) is the fragment count. In the `0x0C` arm, `mov w2, #0x4`
followed by `strb wzr` writes nlohmann's `value_t::boolean` with value `false`,
next to the log string `not need bind, only set wifi`, which is what makes it the
"join WiFi, skip the cloud" switch.

## Still unconfirmed

The GATT characteristic UUIDs are `[inferred]`. They are the SwitchBot standard
pipe and the pair appears verbatim in `bt_bridge`, but nobody has watched them on
the air. The script checks the robot's attribute list and tells you if they are
absent rather than writing into the void.

Whether the BT-MCU requires LE pairing before accepting a write is also open. If
it does, `bluetoothctl` will prompt during connect.
