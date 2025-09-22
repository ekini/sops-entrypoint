package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/getsops/sops/v3/decrypt"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Extracts []struct {
		Path       string `yaml:"path"`
		OutputFile string `yaml:"output_file"`
		EnvVar     string `yaml:"env_var"`
	} `yaml:"extracts"`
}

func main() {
	help := pflag.BoolP("help", "h", false, "Show help")
	envFiles := pflag.StringSlice("env-file", nil, "YAML file with environment variables (can be repeated)")
	pflag.Usage = func() {
		fmt.Println("Usage: sops-entrypoint [flags] <source_file> <config.yaml> <command> [args...]")
		fmt.Println()
		fmt.Println("Decrypt SOPS files and extract values to files/environment variables, then execute command")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  source_file   SOPS-encrypted file to decrypt")
		fmt.Println("  config.yaml   Configuration file with extraction rules")
		fmt.Println("  command       Command to execute with extracted env vars")
		fmt.Println("  args...       Arguments for the command")
		fmt.Println()
		fmt.Println("Flags:")
		pflag.PrintDefaults()
	}
	pflag.Parse()

	if *help {
		pflag.Usage()
		os.Exit(0)
	}

	args := pflag.Args()
	if len(args) < 3 {
		pflag.Usage()
		os.Exit(1)
	}

	sourceFile := args[0]
	configFile := args[1]
	command := args[2:]

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Decrypt the SOPS file
	decryptedData, err := decrypt.File(sourceFile, "yaml")
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
		path    string
		content string
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
				path    string
				content string
			}{extract.OutputFile, valueStr})
		}
	}

	// Write all files
	for _, file := range filesToWrite {
		if err := writeFile(file.path, file.content); err != nil {
			log.Fatalf("Failed to write %s: %v", file.path, err)
		}
		fmt.Printf("Extracted -> %s\n", file.path)
	}

	// Load env files
	for _, envFile := range *envFiles {
		envVarsFromFile, err := loadEnvFile(envFile)
		if err != nil {
			log.Fatalf("Failed to load env file %s: %v", envFile, err)
		}
		envVars = append(envVars, envVarsFromFile...)
	}

	// Execute command
	env := append(os.Environ(), envVars...)

	execPath, err := lookupPath(command[0])
	if err != nil {
		log.Fatalf("Command not found: %v", err)
	}

	if err := syscall.Exec(execPath, command, env); err != nil {
		log.Fatalf("Exec failed: %v", err)
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

func loadEnvFile(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var envData map[string]interface{}
	if err := yaml.Unmarshal(data, &envData); err != nil {
		return nil, err
	}

	var envVars []string
	for k, v := range envData {
		envVars = append(envVars, fmt.Sprintf("%s=%v", k, v))
	}
	return envVars, nil
}

func writeFile(filename, content string) error {
	// Set umask to 077 to disallow access for other users
	oldMask := syscall.Umask(int(0077))
	defer syscall.Umask(oldMask)

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(filename, []byte(content), 0600)
}
