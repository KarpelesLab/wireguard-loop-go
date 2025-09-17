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

When an interface is running, you may use [`wg(8)`](https://git.zx2c4.com/wireguard-tools/about/src/man/wg.8) to configure it.

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