package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetNestedValue(t *testing.T) {
	data := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"key": "nested_value",
		},
		"dot.key": "direct_value",
	}

	tests := []struct {
		path     string
		expected interface{}
	}{
		{"simple", "value"},
		{"nested.key", "nested_value"},
		{"dot.key", "direct_value"},
		{"nonexistent", nil},
		{"nested.nonexistent", nil},
	}

	for _, test := range tests {
		result := getNestedValue(data, test.path)
		if result != test.expected {
			t.Errorf("getNestedValue(%q) = %v, want %v", test.path, result, test.expected)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `extracts:
  - path: "test.path"
    output_file: "test.txt"
    env_var: "TEST_VAR"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if len(config.Extracts) != 1 {
		t.Errorf("Expected 1 extract, got %d", len(config.Extracts))
	}

	extract := config.Extracts[0]
	if extract.Path != "test.path" {
		t.Errorf("Expected path 'test.path', got %q", extract.Path)
	}
	if extract.OutputFile != "test.txt" {
		t.Errorf("Expected output_file 'test.txt', got %q", extract.OutputFile)
	}
	if extract.EnvVar != "TEST_VAR" {
		t.Errorf("Expected env_var 'TEST_VAR', got %q", extract.EnvVar)
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "test.txt")
	content := "test content"

	err := writeFile(testFile, content)
	if err != nil {
		t.Fatalf("writeFile failed: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file mode 0600, got %o", info.Mode().Perm())
	}
}

func TestLookupPath(t *testing.T) {
	// Test absolute path
	result, err := lookupPath("/bin/sh")
	if err != nil {
		t.Errorf("lookupPath('/bin/sh') failed: %v", err)
	}
	if result != "/bin/sh" {
		t.Errorf("Expected '/bin/sh', got %q", result)
	}

	// Test nonexistent command
	_, err = lookupPath("nonexistent_command_12345")
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestLoadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "env.yaml")

	envContent := `DEBUG: "true"
LOG_LEVEL: "info"
PORT: 8080
`

	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	envVars, err := loadEnvFile(envFile)
	if err != nil {
		t.Fatalf("loadEnvFile failed: %v", err)
	}

	if len(envVars) != 3 {
		t.Errorf("Expected 3 env vars, got %d", len(envVars))
	}

	expected := map[string]bool{
		"DEBUG=true":     false,
		"LOG_LEVEL=info": false,
		"PORT=8080":      false,
	}

	for _, envVar := range envVars {
		if _, exists := expected[envVar]; exists {
			expected[envVar] = true
		} else {
			t.Errorf("Unexpected env var: %s", envVar)
		}
	}

	for envVar, found := range expected {
		if !found {
			t.Errorf("Expected env var not found: %s", envVar)
		}
	}
}

func TestLoadEnvFileDuplicateKey(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "env.yaml")

	// A repeated key within a single file is rejected by the YAML parser.
	envContent := `FOO: first
FOO: second
`

	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := loadEnvFile(envFile); err == nil {
		t.Error("Expected error for duplicate key, got nil")
	}
}

func TestLoadEnvFileMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does_not_exist.yaml")

	if _, err := loadEnvFile(missing); err == nil {
		t.Error("Expected error for missing env file, got nil")
	}
}
