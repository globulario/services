package oci

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// NotifySystemd sends one sd_notify datagram. It is a no-op when the process
// was not started with NOTIFY_SOCKET. Abstract sockets are represented by a
// leading '@' in the environment and a leading NUL byte on Linux.
func NotifySystemd(message string) error {
	addr := os.Getenv("NOTIFY_SOCKET")
	if addr == "" {
		return nil
	}
	if strings.HasPrefix(addr, "@") {
		addr = "\x00" + strings.TrimPrefix(addr, "@")
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("dial systemd notify socket: %w", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("write systemd notify message: %w", err)
	}
	return nil
}
