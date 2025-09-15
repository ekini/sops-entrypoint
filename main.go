package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	SourceFile string `yaml:"source_file"`
	Extracts   []struct {
		Path       string `yaml:"path"`
		OutputFile string `yaml:"output_file"`
		EnvVar     string `yaml:"env_var"`
		FileMode   string `yaml:"file_mode"`
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

	// Extract values and prepare for writing
	var envVars []string
	var filesToWrite []struct {
		path     string
		content  string
		fileMode string
	}

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

		// Either env var OR file output
		if extract.EnvVar != "" {
			envVars = append(envVars, fmt.Sprintf("%s=%s", extract.EnvVar, valueStr))
		} else if extract.OutputFile != "" {
			filesToWrite = append(filesToWrite, struct {
				path     string
				content  string
				fileMode string
			}{extract.OutputFile, valueStr, extract.FileMode})
		}
	}

	// Write all files
	for _, file := range filesToWrite {
		if err := writeFile(file.path, file.content, file.fileMode); err != nil {
			log.Fatalf("Failed to write %s: %v", file.path, err)
		}
		fmt.Printf("Extracted -> %s\n", file.path)
	}

	// Execute command if provided
	if len(command) > 0 {
		env := append(os.Environ(), envVars...)
		
		execPath, err := lookupPath(command[0])
		if err != nil {
			log.Fatalf("Command not found: %v", err)
		}
		
		if err := syscall.Exec(execPath, command, env); err != nil {
			log.Fatalf("Exec failed: %v", err)
		}
	}
}

func lookupPath(cmd string) (string, error) {
	if strings.Contains(cmd, "/") {
		return cmd, nil
	}
	
	path := os.Getenv("PATH")
	for _, dir := range strings.Split(path, ":") {
		if dir == "" {
			dir = "."
		}
		execPath := filepath.Join(dir, cmd)
		if info, err := os.Stat(execPath); err == nil && !info.IsDir() {
			return execPath, nil
		}
	}
	return "", fmt.Errorf("executable not found in PATH")
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

	// Try the full path as a single key first
	if val, ok := data[path]; ok {
		return val
	}

	// Fall back to dot-separated navigation
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

func writeFile(filename, content, fileMode string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Set umask if file_mode is specified
	if fileMode != "" {
		if mode, err := strconv.ParseUint(fileMode, 8, 32); err == nil {
			oldMask := syscall.Umask(int(0777 - mode))
			defer syscall.Umask(oldMask)
		}
	}

	return os.WriteFile(filename, []byte(content), 0666)
}
