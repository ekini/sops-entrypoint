package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	SourceFile string `yaml:"source_file"`
	Extracts   []struct {
		Path       string `yaml:"path"`
		OutputFile string `yaml:"output_file"`
		EnvVar     string `yaml:"env_var"`
	} `yaml:"extracts"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: sops-entrypoint <config.yaml> [command...]")
	}

	configFile := os.Args[1]
	command := os.Args[2:]

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Decrypt the SOPS file
	decryptedData, err := decrypt.File(config.SourceFile, "yaml")
	if err != nil {
		log.Fatalf("Failed to decrypt file: %v", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(decryptedData, &data); err != nil {
		log.Fatalf("Failed to parse decrypted data: %v", err)
	}

	// Extract values
	var envVars []string
	for _, extract := range config.Extracts {
		value := getNestedValue(data, extract.Path)
		if value == nil {
			log.Printf("Warning: path %s not found", extract.Path)
			continue
		}

		// Handle dictionary of environment variables
		if dict, ok := value.(map[string]interface{}); ok && extract.EnvVar == "" && extract.OutputFile == "" {
			for k, v := range dict {
				envVars = append(envVars, fmt.Sprintf("%s=%v", k, v))
			}
			continue
		}

		valueStr := fmt.Sprintf("%v", value)

		// Write to file if specified
		if extract.OutputFile != "" {
			if err := writeFile(extract.OutputFile, valueStr); err != nil {
				log.Fatalf("Failed to write %s: %v", extract.OutputFile, err)
			}
			fmt.Printf("Extracted %s -> %s\n", extract.Path, extract.OutputFile)
		}

		// Add to environment if specified
		if extract.EnvVar != "" {
			envVars = append(envVars, fmt.Sprintf("%s=%s", extract.EnvVar, valueStr))
		}
	}

	// Execute command if provided
	if len(command) > 0 {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = append(os.Environ(), envVars...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			log.Fatalf("Command failed: %v", err)
		}
	}
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getNestedValue(data map[string]interface{}, path string) interface{} {
	if path == "" {
		return data
	}

	keys := strings.Split(path, ".")
	current := data
	
	for i, key := range keys {
		if val, ok := current[key]; ok {
			if i == len(keys)-1 {
				return val
			}
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return current
}

func writeFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(content), 0644)
}
