package smb

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"digital.vasic.discovery/pkg/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanner_NilConfig(t *testing.T) {
	s := NewScanner(nil)

	require.NotNil(t, s)
	assert.Equal(t, "smb", s.Protocol())
	assert.Equal(t, []int{445, 139}, s.config.Ports)
	assert.Equal(t, 5*time.Second, s.config.Timeout)
	assert.Equal(t, 50, s.config.MaxConc)
}

func TestNewScanner_EmptyPortsGetDefaults(t *testing.T) {
	cfg := &scanner.Config{
		Network: "192.168.1.0/24",
		Timeout: 3 * time.Second,
		MaxConc: 25,
	}
	s := NewScanner(cfg)

	assert.Equal(t, []int{445, 139}, s.config.Ports)
}

func TestScanner_Protocol(t *testing.T) {
	s := NewScanner(nil)
	assert.Equal(t, "smb", s.Protocol())
}

func TestScanner_Scan_NoNetwork(t *testing.T) {
	s := NewScanner(&scanner.Config{})

	services, err := s.Scan(context.Background())
	assert.Error(t, err)
	assert.Nil(t, services)
	assert.Contains(t, err.Error(), "no network configured")
}

func TestScanner_Scan_InvalidCIDR(t *testing.T) {
	s := NewScanner(&scanner.Config{
		Network: "not-a-cidr",
		Ports:   []int{445},
	})

	services, err := s.Scan(context.Background())
	assert.Error(t, err)
	assert.Nil(t, services)
	assert.Contains(t, err.Error(), "invalid network CIDR")
}

func TestScanner_ScanHost_Unreachable(t *testing.T) {
	cfg := &scanner.Config{
		Timeout: 100 * time.Millisecond,
		Ports:   []int{445},
		MaxConc: 5,
	}
	s := NewScanner(cfg)

	// Use a non-routable IP to ensure connection fails quickly.
	services, err := s.ScanHost(context.Background(), "198.51.100.1")
	assert.NoError(t, err)
	assert.Empty(t, services)
}

func TestScanner_ScanHost_ContextCancelled(t *testing.T) {
	cfg := &scanner.Config{
		Timeout: 5 * time.Second,
		Ports:   []int{445, 139},
		MaxConc: 5,
	}
	s := NewScanner(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	services, err := s.ScanHost(ctx, "192.168.1.1")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, services)
}

func TestScanner_ScanHost_LiveListener(t *testing.T) {
	// Start a local TCP listener to simulate an SMB service.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	// Accept connections in a goroutine so the listener doesn't block.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port

	cfg := &scanner.Config{
		Timeout: 2 * time.Second,
		Ports:   []int{port},
		MaxConc: 5,
	}
	s := NewScanner(cfg)

	services, err := s.ScanHost(context.Background(), "127.0.0.1")
	require.NoError(t, err)
	require.Len(t, services, 1)

	svc := services[0]
	assert.Equal(t, "127.0.0.1", svc.Host)
	assert.Equal(t, port, svc.Port)
	assert.Equal(t, "smb", svc.Protocol)
	assert.Contains(t, svc.Name, "127.0.0.1")
	assert.NotNil(t, svc.Metadata)
	assert.False(t, svc.FoundAt.IsZero())
}

func TestScanner_Scan_WithLiveListener(t *testing.T) {
	// Start a local TCP listener to simulate an SMB service.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port

	// Use a /32 CIDR to only scan localhost.
	cfg := &scanner.Config{
		Network: "127.0.0.1/32",
		Timeout: 2 * time.Second,
		Ports:   []int{port},
		MaxConc: 5,
	}
	s := NewScanner(cfg)

	services, err := s.Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, services, 1)

	assert.Equal(t, "127.0.0.1", services[0].Host)
	assert.Equal(t, port, services[0].Port)
}

func TestScanner_Scan_ContextCancelledDuringScan(t *testing.T) {
	cfg := &scanner.Config{
		Network: "192.168.1.0/24",
		Timeout: 5 * time.Second,
		Ports:   []int{445},
		MaxConc: 5,
	}
	s := NewScanner(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	services, err := s.Scan(ctx)
	// Should return with context error or empty results.
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
	_ = services
}

func TestExpandCIDR_ValidSmall(t *testing.T) {
	hosts, err := expandCIDR("192.168.1.0/30")
	require.NoError(t, err)

	// /30 gives 4 addresses, minus network and broadcast = 2 usable hosts.
	assert.Len(t, hosts, 2)
	assert.Equal(t, "192.168.1.1", hosts[0])
	assert.Equal(t, "192.168.1.2", hosts[1])
}

func TestExpandCIDR_SingleHost(t *testing.T) {
	hosts, err := expandCIDR("10.0.0.5/32")
	require.NoError(t, err)

	// /32 produces exactly 1 address; no stripping.
	assert.Len(t, hosts, 1)
	assert.Equal(t, "10.0.0.5", hosts[0])
}

func TestExpandCIDR_Slash24(t *testing.T) {
	hosts, err := expandCIDR("172.16.0.0/24")
	require.NoError(t, err)

	// /24 = 256 addresses, minus network and broadcast = 254 usable hosts.
	assert.Len(t, hosts, 254)
	assert.Equal(t, "172.16.0.1", hosts[0])
	assert.Equal(t, "172.16.0.254", hosts[len(hosts)-1])
}

func TestExpandCIDR_Invalid(t *testing.T) {
	_, err := expandCIDR("not-a-cidr")
	assert.Error(t, err)
}

func TestPortDescription(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{445, "microsoft-ds"},
		{139, "netbios-ssn"},
		{8080, "unknown"},
		{0, "unknown"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
			assert.Equal(t, tt.expected, portDescription(tt.port))
		})
	}
}

func TestShareInfo_Fields(t *testing.T) {
	info := ShareInfo{
		Name:       "media-share",
		Host:       "192.168.1.10",
		ShareName:  "media",
		ShareType:  "disk",
		Accessible: true,
	}

	assert.Equal(t, "media-share", info.Name)
	assert.Equal(t, "192.168.1.10", info.Host)
	assert.Equal(t, "media", info.ShareName)
	assert.Equal(t, "disk", info.ShareType)
	assert.True(t, info.Accessible)
}

func TestIncrementIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.254").To4()
	incrementIP(ip)
	assert.Equal(t, "192.168.1.255", ip.String())

	incrementIP(ip)
	assert.Equal(t, "192.168.2.0", ip.String())
}

func TestCloneIP(t *testing.T) {
	original := net.ParseIP("10.0.0.1").To4()
	clone := cloneIP(original)

	assert.Equal(t, original.String(), clone.String())

	// Mutating clone should not affect original.
	incrementIP(clone)
	assert.Equal(t, "10.0.0.1", original.String())
	assert.Equal(t, "10.0.0.2", clone.String())
}
