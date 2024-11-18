# keylightctl

`keylightctl` is a CLI tool for managing your Elgato Key Light Air. It allows you to control your lights directly from the terminal by configuring settings in a simple TOML file. With `keylightctl`, you can easily get the status, turn the lights on/off, and adjust brightness and temperature.

## Features

- **Status:** View the current status of your Key Light Air.
- **Power Control:** Turn your light on or off.
- **Brightness Adjustment:** Set brightness to your desired level.
- **Temperature Control:** Adjust the color temperature.

## Configuration

`keylightctl` uses a configuration file located at `$HOME/.keylightctl.toml`. Here’s an example configuration:

```toml
[[lights]]
name = "Left"
ip = "192.168.2.164:9123"

[[lights]]
name = "Right"
ip = "192.168.2.165:9123"
```

## Usage

### Commands

- Get Status:

```sh
keylightctl status
```

- Turn On:

```sh
keylightctl on
```

- Turn Off:

```sh
keylightctl off
```

- Help

For a full list of commands and options:

```sh
keylightctl --help
```
