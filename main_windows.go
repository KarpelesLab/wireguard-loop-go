/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"golang.org/x/sys/windows"

	"github.com/KarpelesLab/wireguard-loop-go/conn"
	"github.com/KarpelesLab/wireguard-loop-go/device"
	"github.com/KarpelesLab/wireguard-loop-go/ipc"
	"github.com/KarpelesLab/wireguard-loop-go/loop"
)

const (
	ExitSetupSuccess = 0
	ExitSetupFailed  = 1
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("wireguard-loop-go v%s\n\nLoop-based WireGuard daemon for Windows.\nBased on wireguard-go by Jason A. Donenfeld.\n", Version)
		return
	}

	var interfaceName string
	if len(os.Args) != 2 {
		os.Stderr.WriteString("Usage: " + os.Args[0] + " <interface name>\n")
		os.Exit(ExitSetupFailed)
	}
	interfaceName = os.Args[1]

	fmt.Fprintln(os.Stderr, "Warning: this software is experimental and has not been security audited.")

	logger := device.NewLogger(
		device.LogLevelError,
		fmt.Sprintf("(%s) ", interfaceName),
	)

	logger.Verbosef("Starting wireguard-loop-go version %s", Version)

	// create loop device (no need for TUN handling anymore)
	tdev, err := loop.CreateLoop(interfaceName, device.DefaultMTU)
	if err != nil {
		logger.Errorf("Failed to create loop device: %v", err)
		os.Exit(ExitSetupFailed)
	}

	realInterfaceName, err := tdev.Name()
	if err == nil {
		interfaceName = realInterfaceName
	}

	device := device.NewDevice(tdev, conn.NewDefaultBind(), logger)

	logger.Verbosef("Device started")

	// open UAPI file (or use supplied fd)

	fileUAPI, err := func() (*os.File, error) {
		uapiFdStr := os.Getenv("WG_UAPI_FD")
		if uapiFdStr == "" {
			return ipc.UAPIOpen(interfaceName)
		}

		// use supplied fd

		fd, err := strconv.ParseUint(uapiFdStr, 10, 32)
		if err != nil {
			return nil, err
		}

		return os.NewFile(uintptr(fd), ""), nil
	}()
	if err != nil {
		logger.Errorf("UAPI listen error: %v", err)
		os.Exit(ExitSetupFailed)
		return
	}

	uapi, err := ipc.UAPIListen(interfaceName, fileUAPI)
	if err != nil {
		logger.Errorf("Failed to listen on uapi socket: %v", err)
		os.Exit(ExitSetupFailed)
	}

	errs := make(chan error)
	term := make(chan os.Signal, 1)

	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				errs <- err
				return
			}
			go device.IpcHandle(conn)
		}
	}()

	logger.Verbosef("UAPI listener started")

	// wait for program to terminate

	signal.Notify(term, os.Interrupt)
	signal.Notify(term, windows.SIGTERM)

	select {
	case <-term:
	case <-errs:
	case <-device.Wait():
	}

	// clean up

	uapi.Close()
	device.Close()

	logger.Verbosef("Shutting down")
}