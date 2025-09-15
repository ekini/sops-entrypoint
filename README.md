# SOPS Entrypoint

A Go utility to decrypt SOPS files and extract specific values to separate files and/or environment variables, then optionally run a command.

## Usage

1. Build the project:
```bash
go build -o sops-entrypoint
```

2. Create a configuration file (see `config.example.yaml`):
```yaml
source_file: "secrets.yaml"
extracts:
  - path: "database.password"
    output_file: "db_password.txt"
    env_var: "DB_PASSWORD"
  - path: "api.key"
    env_var: "API_KEY"
```

3. Run the tool:
```bash
# Extract only
./sops-entrypoint config.yaml

# Extract and run command with environment variables
./sops-entrypoint config.yaml myapp --port 8080
```

## Configuration

- `source_file`: Path to the SOPS-encrypted file
- `extracts`: Array of extractions to perform
  - `path`: Dot-notation path to the value in the decrypted data
  - `output_file`: (Optional) File to write the extracted value to
  - `env_var`: (Optional) Environment variable name to set
  - `file_mode`: (Optional) Octal file permissions (e.g., "0500", "0644")

## Requirements

- SOPS must be configured with appropriate keys for decryption
- The source file must be encrypted with SOPS
