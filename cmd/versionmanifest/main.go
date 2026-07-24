package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	manifestPrefix = "FLASHGATE_BUILD_MANIFEST_V1|"
	manifestSuffix = "|END_FLASHGATE_BUILD_MANIFEST_V1"
)

var manifestKeys = []string{
	"version",
	"fileVersion",
	"commit",
	"sourceTime",
	"modified",
	"goos",
	"goarch",
	"publicArch",
}

func main() {
	binaryPath := flag.String("binary", "", "binary to inspect")
	expected := make(map[string]*string, len(manifestKeys))
	for _, key := range manifestKeys {
		key := key
		expected[key] = flag.String("expected-"+flagName(key), "", "expected "+key)
	}
	flag.Parse()
	if *binaryPath == "" {
		fmt.Fprintln(os.Stderr, "versionmanifest requires --binary")
		os.Exit(2)
	}
	for key, value := range expected {
		if *value == "" {
			fmt.Fprintf(os.Stderr, "versionmanifest requires --expected-%s\n", flagName(key))
			os.Exit(2)
		}
	}

	data, err := os.ReadFile(*binaryPath)
	if err != nil {
		fail(err)
	}
	actual, err := parseManifest(data)
	if err != nil {
		fail(err)
	}
	for key, value := range expected {
		if actual[key] != *value {
			fail(fmt.Errorf("embedded build manifest mismatch for %s", key))
		}
	}

	fmt.Println("Status: PASS")
	fmt.Printf("BinaryPath: %s\n", *binaryPath)
	for _, key := range manifestKeys {
		fmt.Printf("%s: %s\n", key, actual[key])
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "versionmanifest: %v\n", err)
	os.Exit(1)
}

func flagName(key string) string {
	var builder strings.Builder
	for _, character := range key {
		if character >= 'A' && character <= 'Z' {
			builder.WriteByte('-')
			character += 'a' - 'A'
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

func parseManifest(data []byte) (map[string]string, error) {
	prefix := []byte(manifestPrefix)
	if bytes.Count(data, prefix) != 1 {
		return nil, errors.New("binary must contain exactly one build-manifest prefix")
	}
	start := bytes.Index(data, prefix) + len(prefix)
	remainder := data[start:]
	suffix := []byte(manifestSuffix)
	end := bytes.Index(remainder, suffix)
	if end < 0 {
		return nil, errors.New("binary build manifest is not terminated")
	}
	body := string(remainder[:end])
	if strings.Contains(body, "\x00") {
		return nil, errors.New("binary build manifest contains a NUL byte")
	}

	result := make(map[string]string, len(manifestKeys))
	for _, field := range strings.Split(body, "|") {
		key, value, found := strings.Cut(field, "=")
		if !found || key == "" || value == "" {
			return nil, errors.New("binary build manifest contains an invalid field")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate build-manifest field %q", key)
		}
		result[key] = value
	}

	actualKeys := make([]string, 0, len(result))
	for key := range result {
		actualKeys = append(actualKeys, key)
	}
	sort.Strings(actualKeys)
	expectedKeys := append([]string(nil), manifestKeys...)
	sort.Strings(expectedKeys)
	if strings.Join(actualKeys, "\x00") != strings.Join(expectedKeys, "\x00") {
		return nil, errors.New("binary build manifest has unexpected fields")
	}
	return result, nil
}
