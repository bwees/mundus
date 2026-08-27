# Mundus

Local control for the SwitchBot S20 robot vacuum. Home Assistant MQTT
integration, a web UI, and full offline operation (with OS level blocking of Cloud Connectivity).

## Installing

See the [USB Installer Documentation](usb/README.md) for how to install Mundus via USB developer mode.

## Local Control UI

Mundus offers a local control UI on `http://<ROBOT-IP>:8080` that allows editing all settings, map, zones, local SSH access, MQTT server (for Home Assistant integration), and control. This is accessible after installing the Mundus USB installer and rebooting the robot.

Once Mundus is installed, please set an admin password in the web UI to prevent unauthorized access. 

## Home Assistant Integration

Home Assistant can integrate with Mundus via MQTT. Enter your MQTT server and credentials in the Mundus web UI (see above) under the "System" tab. The device will be automatically discovered in Home Assistant.

## Turning off Cloud Connectivity

By default, Mundus does not block cloud connectivity. You can continue to use the SwitchBot app and cloud services alongside Mundus if you choose. 

You can block the robot's cloud connectivity entirely via the system tab in the web UI. This will prevent the robot from connecting to SwitchBot's cloud services, and it will operate entirely locally.

## Updates

There is a "Check for Updates" button in the web UI. Mundus checks GitHub releases for a new release, downloads it, and applies it on the next boot. It does not check for updates automatically.

### Disclaimer

This project was tested on my personal SwitchBot S20 robot vacuum. There may be differences in other units, and I cannot guarantee that this will work on all SwitchBot S20 units. Please proceed with caution and at your own risk. I am not responsible for any damage or issues that may arise from using this software.

Please note, Claude was heavily used in the reverse engineering of the SwitchBot firmware and control protocols. The application was written with the help of Claude but I was significantly involved in the design, testing, and implementation of the project.