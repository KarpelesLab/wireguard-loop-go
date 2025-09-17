# wireguard-loop-go

A loop-based implementation of WireGuard that operates without requiring tun device privileges.

Based on the official [WireGuard](https://www.wireguard.com/) Go implementation.

## What is wireguard-loop-go?

This is a modified version of wireguard-go that replaces the tun device with a loop device implementation. Instead of creating network interfaces that require root/administrator privileges, this implementation creates a loop device that simply echoes back any packet it receives. This makes it suitable for:

- Testing WireGuard protocol implementations
- Running as a regular user without elevated privileges
- Development and debugging purposes
- Learning about WireGuard internals

## Usage

```
$ wireguard-loop-go wg0
```

This will create a loop device interface with the specified name. The interface operates entirely in userspace and doesn't require special privileges.

### UAPI Socket Location

The UAPI socket (used by `wg` command) is created at:
- **With XDG_RUNTIME_DIR**: `$XDG_RUNTIME_DIR/wireguard-loop/wg0.sock` (typically `/run/user/1000/wireguard-loop/wg0.sock`)
- **Without XDG_RUNTIME_DIR**: `/tmp/wireguard-loop/wg0.sock`
- **With WG_SOCKET_DIR env var**: `$WG_SOCKET_DIR/wg0.sock` (for custom locations)

Since the standard `wg` tool looks for sockets in `/var/run/wireguard/`, you may need to create a symlink with sudo (which preserves your environment variables):
```
$ sudo mkdir -p /var/run/wireguard
$ sudo ln -s $XDG_RUNTIME_DIR/wireguard-loop/wg0.sock /var/run/wireguard/wg0.sock
```

After this, you can use [`wg(8)`](https://git.zx2c4.com/wireguard-tools/about/src/man/wg.8) to configure the interface normally.

To run with more logging you may set the environment variable `LOG_LEVEL=debug`.

## Building

This requires an installation of the latest version of [Go](https://go.dev/).

```
$ git clone https://github.com/KarpelesLab/wireguard-loop-go
$ cd wireguard-loop-go
$ go build -o wireguard-loop-go
```

## How it works

Unlike the standard wireguard-go which uses a tun device to interface with the kernel's networking stack, wireguard-loop-go implements a simple loop device that:

1. Receives packets from the WireGuard protocol layer
2. Immediately sends them back to the sender
3. Operates entirely in userspace without kernel interaction

This makes it a lightweight testing tool that can run without special privileges.

## License

This project is licensed under the MIT License, same as the original wireguard-go.

## Credits

Based on wireguard-go by Jason A. Donenfeld and the WireGuard contributors.