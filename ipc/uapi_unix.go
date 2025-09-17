//go:build linux || darwin || freebsd || openbsd

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

const (
	IpcErrorIO        = -int64(unix.EIO)
	IpcErrorProtocol  = -int64(unix.EPROTO)
	IpcErrorInvalid   = -int64(unix.EINVAL)
	IpcErrorPortInUse = -int64(unix.EADDRINUSE)
	IpcErrorUnknown   = -55 // ENOANO
)

// socketDirectory is variable because it is modified by a linker
// flag in wireguard-android.
// For wireguard-loop-go, we use a user-writable location by default
var socketDirectory = func() string {
	// Try XDG_RUNTIME_DIR first (standard for user runtime files)
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/wireguard-loop"
	}
	// Fall back to temp directory
	return os.TempDir() + "/wireguard-loop"
}()

func sockPath(iface string) string {
	return fmt.Sprintf("%s/%s.sock", socketDirectory, iface)
}

func UAPIOpen(name string) (*os.File, error) {
	if err := os.MkdirAll(socketDirectory, 0o755); err != nil {
		return nil, err
	}

	socketPath := sockPath(name)
	addr, err := net.ResolveUnixAddr("unix", socketPath)
	if err != nil {
		return nil, err
	}

	oldUmask := unix.Umask(0o077)
	defer unix.Umask(oldUmask)

	listener, err := net.ListenUnix("unix", addr)
	if err == nil {
		return listener.File()
	}

	// Test socket, if not in use cleanup and try again.
	if _, err := net.Dial("unix", socketPath); err == nil {
		return nil, errors.New("unix socket in use")
	}
	if err := os.Remove(socketPath); err != nil {
		return nil, err
	}
	listener, err = net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	return listener.File()
}
