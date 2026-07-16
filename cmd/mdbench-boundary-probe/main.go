package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

const listenerAddress = "127.0.0.1:18080"

type result struct {
	WorkspaceWrite bool `json:"workspace_write"`
	RootWrite      bool `json:"root_write"`
	ControlRead    bool `json:"control_read"`
	CredentialRead bool `json:"credential_read"`
	HostRead       bool `json:"host_read"`
	NetworkConnect bool `json:"network_connect"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "serve" {
		serve()
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "network" {
		if canConnect(listenerAddress) {
			return
		}
		os.Exit(1)
	}
	value := result{
		WorkspaceWrite: canWrite("/work/.mdbench-boundary-probe"),
		RootWrite:      canWrite("/etc/.mdbench-boundary-probe"),
		ControlRead:    canRead("/control/public.txt"),
		CredentialRead: canRead("/codex-home/auth.json"),
		HostRead:       canRead("/host-home/.ssh"),
		NetworkConnect: canConnect(listenerAddress),
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() {
	listener, err := net.Listen("tcp", listenerAddress)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer listener.Close()
	for {
		connection, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = connection.Close()
	}
}

func canWrite(path string) bool {
	if err := os.WriteFile(path, []byte("probe"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(path)
	return true
}

func canRead(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	return file.Close() == nil
}

func canConnect(address string) bool {
	connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	return connection.Close() == nil
}
