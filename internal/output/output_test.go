package output

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestSuccessFlatMap(t *testing.T) {
	stdout := captureStdout(t, func() {
		Success(map[string]any{"operation": "version", "version": "1.0"})
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimLine(stdout)), &payload); err != nil {
		t.Fatalf("输出不是单行 JSON: %q %v", stdout, err)
	}
	if payload["ok"] != true || payload["version"] != "1.0" {
		t.Errorf("载荷不符: %+v", payload)
	}
}

func TestSuccessWrapsStruct(t *testing.T) {
	type demo struct {
		Value string `json:"value"`
	}
	stdout := captureStdout(t, func() {
		Success(demo{Value: "x"})
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimLine(stdout)), &payload); err != nil {
		t.Fatalf("输出不是单行 JSON: %q %v", stdout, err)
	}
	if payload["ok"] != true {
		t.Errorf("缺少 ok 字段: %+v", payload)
	}
	if payload["value"] != "x" {
		t.Errorf("结构体字段应平铺: %+v", payload)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = original }()
	fn()
	writer.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func trimLine(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
