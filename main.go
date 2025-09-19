//go:build !windows

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
	"github.com/KarpelesLab/wireguard-loop-go/conn"
	"github.com/KarpelesLab/wireguard-loop-go/device"
	"github.com/KarpelesLab/wireguard-loop-go/ipc"
	"github.com/KarpelesLab/wireguard-loop-go/loop"
)

const (
	ExitSetupSuccess = 0
	ExitSetupFailed  = 1
)

const (
	ENV_WG_UAPI_FD            = "WG_UAPI_FD"
	ENV_WG_UAPI_EP_FD         = "WG_UAPI_EP_FD"
	ENV_WG_PROCESS_FOREGROUND = "WG_PROCESS_FOREGROUND"
)

func printUsage() {
	fmt.Printf("Usage: %s [-f/--foreground] INTERFACE-NAME\n", os.Args[0])
}

func warning() {
	// No warning needed for wireguard-loop-go since it serves a different purpose
	// (loop device testing rather than replacing kernel module)
	return
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("wireguard-loop-go v%s\n\nLoop-based WireGuard daemon for %s-%s.\nBased on wireguard-go by Jason A. Donenfeld.\n", Version, runtime.GOOS, runtime.GOARCH)
		return
	}

	warning()

	var foreground bool
	var interfaceName string
	if len(os.Args) < 2 || len(os.Args) > 3 {
		printUsage()
		return
	}

	switch os.Args[1] {

	case "-f", "--foreground":
		foreground = true
		if len(os.Args) != 3 {
			printUsage()
			return
		}
		interfaceName = os.Args[2]

	default:
		foreground = false
		if len(os.Args) != 2 {
			printUsage()
			return
		}
		interfaceName = os.Args[1]
	}

	if !foreground {
		foreground = os.Getenv(ENV_WG_PROCESS_FOREGROUND) == "1"
	}

	// get log level (default: info)

	logLevel := func() int {
		switch os.Getenv("LOG_LEVEL") {
		case "verbose", "debug":
			return device.LogLevelVerbose
		case "error":
			return device.LogLevelError
		case "silent":
			return device.LogLevelSilent
		}
		return device.LogLevelError
	}()

	// create loop device (no need for TUN_FD anymore)

	tdev, err := loop.CreateLoop(interfaceName, device.DefaultMTU)

	if err == nil {
		realInterfaceName, err2 := tdev.Name()
		if err2 == nil {
			interfaceName = realInterfaceName
		}
	}

	logger := device.NewLogger(
		logLevel,
		fmt.Sprintf("(%s) ", interfaceName),
	)

	logger.Verbosef("Starting wireguard-loop-go version %s", Version)

	if err != nil {
		logger.Errorf("Failed to create loop device: %v", err)
		os.Exit(ExitSetupFailed)
	}

	// open UAPI file (or use supplied fd)

	var fileUAPI *os.File
	var isEndpointFd bool

	// Check for connected endpoint fd first
	if uapiEpFdStr := os.Getenv(ENV_WG_UAPI_EP_FD); uapiEpFdStr != "" {
		fd, err := strconv.ParseUint(uapiEpFdStr, 10, 32)
		if err != nil {
			logger.Errorf("Invalid WG_UAPI_EP_FD: %v", err)
			os.Exit(ExitSetupFailed)
			return
		}
		fileUAPI = os.NewFile(uintptr(fd), "")
		isEndpointFd = true
	} else if uapiFdStr := os.Getenv(ENV_WG_UAPI_FD); uapiFdStr != "" {
		// use supplied listener fd
		fd, err := strconv.ParseUint(uapiFdStr, 10, 32)
		if err != nil {
			logger.Errorf("Invalid WG_UAPI_FD: %v", err)
			os.Exit(ExitSetupFailed)
			return
		}
		fileUAPI = os.NewFile(uintptr(fd), "")
		isEndpointFd = false
	} else {
		// create new listener
		var err error
		fileUAPI, err = ipc.UAPIOpen(interfaceName)
		if err != nil {
			logger.Errorf("UAPI listen error: %v", err)
			os.Exit(ExitSetupFailed)
			return
		}
		isEndpointFd = false
	}

	// daemonize the process

	if !foreground && !isEndpointFd {
		// Don't daemonize if we're using an endpoint fd
		env := os.Environ()
		env = append(env, fmt.Sprintf("%s=3", ENV_WG_UAPI_FD))
		env = append(env, fmt.Sprintf("%s=1", ENV_WG_PROCESS_FOREGROUND))
		files := [3]*os.File{}
		if os.Getenv("LOG_LEVEL") != "" && logLevel != device.LogLevelSilent {
			files[0], _ = os.Open(os.DevNull)
			files[1] = os.Stdout
			files[2] = os.Stderr
		} else {
			files[0], _ = os.Open(os.DevNull)
			files[1], _ = os.Open(os.DevNull)
			files[2], _ = os.Open(os.DevNull)
		}
		attr := &os.ProcAttr{
			Files: []*os.File{
				files[0], // stdin
				files[1], // stdout
				files[2], // stderr
				fileUAPI,
			},
			Dir: ".",
			Env: env,
		}

		path, err := os.Executable()
		if err != nil {
			logger.Errorf("Failed to determine executable: %v", err)
			os.Exit(ExitSetupFailed)
		}

		process, err := os.StartProcess(
			path,
			os.Args,
			attr,
		)
		if err != nil {
			logger.Errorf("Failed to daemonize: %v", err)
			os.Exit(ExitSetupFailed)
		}
		process.Release()
		return
	}

	device := device.NewDevice(tdev, conn.NewDefaultBind(), logger)

	logger.Verbosef("Device started")

	errs := make(chan error)
	term := make(chan os.Signal, 1)

	if isEndpointFd {
		// Handle single connected socket endpoint
		logger.Verbosef("Using connected endpoint socket")

		// Create a net.Conn from the file descriptor
		conn, err := net.FileConn(fileUAPI)
		if err != nil {
			logger.Errorf("Failed to create connection from endpoint fd: %v", err)
			os.Exit(ExitSetupFailed)
		}

		// Handle the single connection in a goroutine
		go func() {
			device.IpcHandle(conn)
			// When connection closes, signal exit
			errs <- fmt.Errorf("endpoint connection closed")
		}()
	} else {
		// Handle listener socket (existing behavior)
		uapi, err := ipc.UAPIListen(interfaceName, fileUAPI)
		if err != nil {
			logger.Errorf("Failed to listen on uapi socket: %v", err)
			os.Exit(ExitSetupFailed)
		}
		defer uapi.Close()

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
	}

	// wait for program to terminate

	signal.Notify(term, unix.SIGTERM)
	signal.Notify(term, os.Interrupt)

	select {
	case <-term:
	case err := <-errs:
		if isEndpointFd {
			logger.Verbosef("Endpoint closed: %v", err)
		}
	case <-device.Wait():
	}

	// clean up

	device.Close()

	logger.Verbosef("Shutting down")
}