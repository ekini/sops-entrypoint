package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

func TestSOPSDecryption(t *testing.T) {
	// Set AGE key for decryption
	keyPath := filepath.Join("testdata", "test.key")
	os.Setenv("SOPS_AGE_KEY_FILE", keyPath)
	defer os.Unsetenv("SOPS_AGE_KEY_FILE")

	// Test decryption
	decryptedData, err := decrypt.File("testdata/test_secrets_encrypted.yaml", "yaml")
	if err != nil {
		t.Fatalf("Failed to decrypt SOPS file: %v", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(decryptedData, &data); err != nil {
		t.Fatalf("Failed to parse decrypted YAML: %v", err)
	}

	// Verify decrypted content
	if data["simple_value"] != "test_value" {
		t.Errorf("Expected simple_value 'test_value', got %v", data["simple_value"])
	}

	// Test nested value extraction
	dbPassword := getNestedValue(data, "database.password")
	if dbPassword != "secret123" {
		t.Errorf("Expected database.password 'secret123', got %v", dbPassword)
	}

	apiKey := getNestedValue(data, "api.key")
	if apiKey != "api_key_456" {
		t.Errorf("Expected api.key 'api_key_456', got %v", apiKey)
	}
}

func TestFullWorkflow(t *testing.T) {
	// Set AGE key for decryption
	keyPath := filepath.Join("testdata", "test.key")
	os.Setenv("SOPS_AGE_KEY_FILE", keyPath)
	defer os.Unsetenv("SOPS_AGE_KEY_FILE")

	// Create test config
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `extracts:
  - path: "database.password"
    output_file: "db_pass.txt"
    env_var: "DB_PASS"
  - path: "api.key"
    env_var: "API_KEY"
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Decrypt SOPS file
	decryptedData, err := decrypt.File("testdata/test_secrets_encrypted.yaml", "yaml")
	if err != nil {
		t.Fatalf("Failed to decrypt file: %v", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(decryptedData, &data); err != nil {
		t.Fatalf("Failed to parse decrypted data: %v", err)
	}

	// Process extracts
	var envVars []string
	for _, extract := range config.Extracts {
		value := getNestedValue(data, extract.Path)
		if value == nil {
			t.Errorf("Path %s not found", extract.Path)
			continue
		}

		valueStr := string(value.(string))

		if extract.EnvVar != "" {
			envVars = append(envVars, extract.EnvVar+"="+valueStr)
		}

		if extract.OutputFile != "" {
			outputPath := filepath.Join(tmpDir, extract.OutputFile)
			if err := writeFile(outputPath, valueStr); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			// Verify file content
			content, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("Failed to read output file: %v", err)
			}
			if string(content) != valueStr {
				t.Errorf("File content mismatch: got %q, want %q", string(content), valueStr)
			}
		}
	}

	// Verify environment variables
	expectedEnvVars := []string{"DB_PASS=secret123", "API_KEY=api_key_456"}
	if len(envVars) != len(expectedEnvVars) {
		t.Errorf("Expected %d env vars, got %d", len(expectedEnvVars), len(envVars))
	}

	for i, expected := range expectedEnvVars {
		if i < len(envVars) && envVars[i] != expected {
			t.Errorf("Expected env var %q, got %q", expected, envVars[i])
		}
	}
}
