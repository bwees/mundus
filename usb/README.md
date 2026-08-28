# USB Installer

## Get the zip

Download `develop-tools-mundus.zip` from the
[latest release](https://github.com/bwees/mundus/releases/latest), or from the
artifacts of any CI run on `main`.

## Put it on a drive

The flash drive must be setup in a specific way for the installer to work:

1. An MBR partition table with one primary FAT32 partition enumerating as
   `/dev/sda1`. Any other filesystem or disk layout will fail to copy.
   - macOS
      - You can use Disk Utility to format your disk as "FAT" (aka FAT32) and select Master Boot Record
         <img width="493" height="270" alt="image" src="https://github.com/user-attachments/assets/cbd00a99-71e3-40b8-bfac-5137a6627aed" />
   - Windows
      - Use [Rufus](https://rufus.ie/en/)
      - Select your USB drive under Device.
      - For Boot selection, choose Non bootable.
      - Set Partition scheme to MBR.
      - Set File system to FAT32.
      - Click START.
   - Linux - use whatever your favorite tool is :)
2. `/developer/` must contain exactly one file, the zip. Any extra files/directories cause the copy to fail. macOS may sometimes mut dotfiles in the directory. Before ejecting, run ```dot_clean -m /Volumes/<YOUR DRIVE NAME>``` to clean the drive

Your drive contents should look like this before plugging it into 

```
<FAT32 sda1>/
└── developer/
    └── develop-tools-mundus.zip     # exactly one file
```

## Installing mundus

First, turn off the robot via its switch under the magnetic cover.

The SwitchBot S20 has an easily accessible USB-C OTG port on top of it. It can be accessed by prying up the cover around the buttons on top. There are a few clips and it takes some force to remove it. Once removed, you will see a USB-C port exposed on top.

<img height="400" alt="image" src="https://github.com/user-attachments/assets/c649b4c7-f7d8-4ccd-9263-a15144af08f4" />

Plug in a USB-C hub into the port and plug your flash drive into that. Turn the robot's switch on and wait. You will hear a speaking audio clip (from what I can tell it says "Starting Developer Mode" in Chinese). This audio gets stopped midway for some reason on my robot. 

On a computer, go to the robot's control interface at `http://<ROBOT-IP>:8080`. Keep refreshing until the page loads. **Do not turn off the robot until the page loads with the admin password setup page.** Turning the robot off during this process may put the OS into a state where Mundus cannot be installed without a full factory reset.

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

