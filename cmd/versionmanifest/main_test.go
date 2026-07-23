package main

import (
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version=1.2.3-rc.1",
		"fileVersion=1.2.3.0",
		"commit=0123456789012345678901234567890123456789",
		"sourceTime=2026-07-23T12:00:00Z",
		"modified=false",
		"goos=windows",
		"goarch=arm64",
		"publicArch=arm64",
	}, "|")
	data := []byte("before" + manifestPrefix + body + manifestSuffix + "after")
	manifest, err := parseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if manifest["version"] != "1.2.3-rc.1" {
		t.Fatalf("unexpected version %q", manifest["version"])
	}
}

func TestParseManifestRejectsMissingDuplicateAndUnknownData(t *testing.T) {
	t.Parallel()
	validBody := strings.Join([]string{
		"version=1.2.3",
		"fileVersion=1.2.3.0",
		"commit=0123456789012345678901234567890123456789",
		"sourceTime=2026-07-23T12:00:00Z",
		"modified=false",
		"goos=windows",
		"goarch=amd64",
		"publicArch=x64",
	}, "|")
	for name, data := range map[string][]byte{
		"missing":   []byte("no manifest"),
		"duplicate": []byte(manifestPrefix + validBody + manifestSuffix + manifestPrefix),
		"unknown": []byte(
			manifestPrefix + validBody + "|unexpected=value" + manifestSuffix,
		),
	} {
		data := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseManifest(data); err == nil {
				t.Fatal("invalid manifest unexpectedly passed")
			}
		})
	}
}

func TestFlagName(t *testing.T) {
	t.Parallel()
	if actual := flagName("publicArch"); actual != "public-arch" {
		t.Fatalf("unexpected flag name %q", actual)
	}
}
