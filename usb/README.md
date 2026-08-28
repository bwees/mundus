# USB Installer

## Get the zip

Download `develop-tools-mundus.zip` from the
[latest release](https://github.com/bwees/mundus/releases/latest), or from the
artifacts of any CI run on `main`.

## Put it on a drive

The flash drive must be setup in a specific way for the installer to work:

1. An MBR partition table with one primary FAT32 partition enumerating as
   `/dev/sda1`. Any other filesystem or disk layout will fail to copy
2. `/developer/` must contain exactly one file, the zip. Any extra files/directories cause the copy to fail, including macOS `._*`, `.Trashes` and
   `.Spotlight-V100`, and Windows `System Volume Information`. Formatting and
   authoring on Linux or Windows avoids most of this.

```
<FAT32 sda1>/
└── developer/
    └── develop-tools-mundus.zip     # exactly one file
```

## Installing mundus

First, turn off the robot via its switch under the magnetic cover.

The SwitchBot S20 has an easily accessible USB-C OTG port on top of it. It can be accessed by prying up the cover around the buttons on top. There are a few clips and it takes some force to remove it. Once removed, you will see a USB-C port exposed on top.

<img height="400" alt="image" src="https://github.com/user-attachments/assets/c649b4c7-f7d8-4ccd-9263-a15144af08f4" />

Plug in a USB-C hub into the port and plug your flash drive into that. Turn the robot's switch on and wait.

You will hear a speaking audio clip (from what I can tell it says "Starting Developer Mode" in Chinese). This audio gets stopped midway for some reason on my robot. I waited about 30 seconds then turned the robot off via its power switch. Remove the USB-C hub, place the cover back on, and turn the robot back on.

The mundus web interface will now be accessible at `http://<ROBOT-IP>:8080` once the robot is done booting.

## Build zip locally (developer only)

Needs a cross-compiled server binary and a built web UI:

```sh
mise run dist       # dist/mundus (linux/arm64) and dist/web
mise run zip        # dist/develop-tools-mundus.zip
```

Or call the script directly, which is what CI does:

```sh
./usb/build-zip.sh --bin dist/mundus --web web/build --out dist/develop-tools-mundus.zip
```

