package core

import (
	"encoding/json"
	"strings"
	"testing"

	panel "github.com/ZicBoard/ZicNode/api/zicboard"
	coreConf "github.com/xtls/xray-core/infra/conf"
)

func TestEffectiveTransportNetwork(t *testing.T) {
	tests := map[string]string{
		"":        "tcp",
		"TCP":     "tcp",
		"http":    "httpupgrade",
		"xhttp":   "xhttp",
		"grpc":    "grpc",
		"ws":      "ws",
		"unknown": "unknown",
	}

	for input, expected := range tests {
		if got := effectiveTransportNetwork(input); got != expected {
			t.Fatalf("effectiveTransportNetwork(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestUnmarshalNetworkSettingsAcceptsEmptyObjectAndLegacyEmptyArray(t *testing.T) {
	nodeInfo := testNodeInfo()

	var tcpFromObject *coreConf.TCPConfig
	if err := unmarshalNetworkSettings(json.RawMessage(`{}`), &tcpFromObject, "tcp", nodeInfo); err != nil {
		t.Fatalf("empty object returned error: %v", err)
	}
	if tcpFromObject == nil {
		t.Fatal("empty object should allocate TCP settings")
	}

	var tcpFromArray *coreConf.TCPConfig
	if err := unmarshalNetworkSettings(json.RawMessage(`[]`), &tcpFromArray, "tcp", nodeInfo); err != nil {
		t.Fatalf("legacy empty array returned error: %v", err)
	}
	if tcpFromArray == nil {
		t.Fatal("legacy empty array should be treated as empty object")
	}
}

func TestUnmarshalNetworkSettingsRejectsNonEmptyArray(t *testing.T) {
	err := unmarshalNetworkSettings(json.RawMessage(`[{"path":"/"}]`), &coreConf.TCPConfig{}, "tcp", testNodeInfo())
	if err == nil {
		t.Fatal("non-empty array should be rejected")
	}
	if !strings.Contains(err.Error(), "node_id=123") || !strings.Contains(err.Error(), "network=tcp") {
		t.Fatalf("error should include node context, got: %v", err)
	}
}

func testNodeInfo() *panel.NodeInfo {
	return &panel.NodeInfo{
		Id: 123,
		Common: &panel.CommonNode{
			Protocol: "vless",
			Network:  "tcp",
		},
	}
}
