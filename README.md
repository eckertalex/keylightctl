# klair

Controls Elgato Key Light Air devices from the terminal: get status, turn lights
on and off, and set brightness and temperature. Pure C (C11 + POSIX sockets, no
dependencies), written handmade-hero style as a learning exercise — see
[WALKTHROUGH.md](WALKTHROUGH.md) for a section-by-section explanation of how it
works.

Lights are read from `~/.config/klair`, one `name = ip` pair per line:

```
left  = 192.168.2.164:9123
right = 192.168.2.165:9123
```

```sh
klair             # no command: same as status
klair status      # show each light's status
klair on          # turn on (-b brightness, -t temperature, -l light)
klair off         # turn off (-l light)
klair --version   # print version
```

Build and install with make:

```sh
make              # ./klair (optimized release build)
make debug        # ./klair_debug (AddressSanitizer + UBSan)
make test         # build and run the test suite
make install      # to ~/.local/bin (override with INSTALL_DIR=)
make uninstall    # remove from INSTALL_DIR
```
