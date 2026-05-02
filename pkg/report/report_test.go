package report

import (
	"encoding/json"
	"testing"
	"time"

	"digital.vasic.discovery/pkg/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleServices() []*scanner.Service {
	now := time.Now()
	return []*scanner.Service{
		{
			Name:     "smb-192.168.1.10:445",
			Host:     "192.168.1.10",
			Port:     445,
			Protocol: "smb",
			Metadata: map[string]string{"port_type": "microsoft-ds"},
			FoundAt:  now,
		},
		{
			Name:     "smb-192.168.1.10:139",
			Host:     "192.168.1.10",
			Port:     139,
			Protocol: "smb",
			Metadata: map[string]string{"port_type": "netbios-ssn"},
			FoundAt:  now,
		},
		{
			Name:     "smb-192.168.1.20:445",
			Host:     "192.168.1.20",
			Port:     445,
			Protocol: "smb",
			Metadata: map[string]string{"port_type": "microsoft-ds"},
			FoundAt:  now,
		},
	}
}

func TestNewReport(t *testing.T) {
	services := sampleServices()
	duration := 2 * time.Second

	report := NewReport("192.168.1.0/24", services, duration)

	require.NotNil(t, report)
	assert.Equal(t, "192.168.1.0/24", report.Network)
	assert.Equal(t, 3, report.TotalFound)
	assert.Equal(t, 2*time.Second, report.Duration)
	assert.Len(t, report.Services, 3)
	assert.False(t, report.ScanTime.IsZero())
}

func TestReport_ToJSON(t *testing.T) {
	services := sampleServices()
	report := NewReport("192.168.1.0/24", services, 1500*time.Millisecond)

	data, err := report.ToJSON()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's valid JSON by unmarshalling.
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.0/24", parsed["network"])
	assert.Equal(t, float64(3), parsed["total_found"])

	servicesArr, ok := parsed["services"].([]interface{})
	require.True(t, ok)
	assert.Len(t, servicesArr, 3)
}

func TestReport_ToJSON_Empty(t *testing.T) {
	report := NewReport("10.0.0.0/8", nil, 0)

	data, err := report.ToJSON()
	require.NoError(t, err)

	var parsed Report
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 0, parsed.TotalFound)
	assert.Nil(t, parsed.Services)
}

func TestReport_ToJSON_Roundtrip(t *testing.T) {
	services := sampleServices()
	original := NewReport("192.168.1.0/24", services, 3*time.Second)

	data, err := original.ToJSON()
	require.NoError(t, err)

	var restored Report
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.Network, restored.Network)
	assert.Equal(t, original.TotalFound, restored.TotalFound)
	assert.Len(t, restored.Services, len(original.Services))

	for i, svc := range restored.Services {
		assert.Equal(t, original.Services[i].Host, svc.Host)
		assert.Equal(t, original.Services[i].Port, svc.Port)
		assert.Equal(t, original.Services[i].Protocol, svc.Protocol)
	}
}

func TestReport_Summary_WithServices(t *testing.T) {
	services := sampleServices()
	report := NewReport("192.168.1.0/24", services, 2*time.Second)

	summary := report.Summary()

	assert.Contains(t, summary, "Discovery Report")
	assert.Contains(t, summary, "192.168.1.0/24")
	assert.Contains(t, summary, "3 service(s)")
	assert.Contains(t, summary, "[smb]")
	assert.Contains(t, summary, "192.168.1.10:445")
	assert.Contains(t, summary, "192.168.1.10:139")
	assert.Contains(t, summary, "192.168.1.20:445")
}

func TestReport_Summary_Empty(t *testing.T) {
	report := NewReport("10.0.0.0/24", nil, 100*time.Millisecond)

	summary := report.Summary()

	assert.Contains(t, summary, "Discovery Report")
	assert.Contains(t, summary, "10.0.0.0/24")
	assert.Contains(t, summary, "0 service(s)")
	assert.NotContains(t, summary, "Services:")
}

func TestReport_Summary_Duration(t *testing.T) {
	report := NewReport("172.16.0.0/16", nil, 1234*time.Millisecond)

	summary := report.Summary()

	assert.Contains(t, summary, "1.234s")
}
