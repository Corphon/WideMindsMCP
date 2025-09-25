package utils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads a YAML file from the provided path into the target structure.
func LoadYAML(path string, target interface{}) error {
	if target == nil {
		return errors.New("target must not be nil")
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open yaml: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode yaml: %w", err)
	}

	return nil
}

// LoadEnvFile parses a simple KEY=VALUE env file and applies values to the process environment.
func LoadEnvFile(path string) (map[string]string, error) {
	result := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		result[key] = value
		_ = os.Setenv(key, value)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// ResolveConfigPath ensures the config path is absolute, resolving relative paths against the current working directory.
func ResolveConfigPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is empty")
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, path), nil
}
