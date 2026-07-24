package main

import (
	"strings"
	"testing"
)

func TestBuildConfig(t *testing.T) {
	cfg, err := buildConfig(options{
		productVersion: "1.2.3-rc.1",
		sourceTime:     "2026-07-21T16:00:00Z",
		goarch:         "amd64",
		output:         "resource_windows_amd64.syso",
		icon:           "flashgate.ico",
	})
	if err != nil {
		t.Fatalf("buildConfig returned error: %v", err)
	}

	if cfg.FileVersion != "1.2.3.0" {
		t.Fatalf("unexpected file version %q", cfg.FileVersion)
	}
	if cfg.ProductVersion != "1.2.3-rc.1" {
		t.Fatalf("unexpected product version %q", cfg.ProductVersion)
	}
	if cfg.CompanyName != "Thomas Weidner" {
		t.Fatalf("unexpected company %q", cfg.CompanyName)
	}
	if cfg.TranslationID != 0x0409 || cfg.CharsetID != 1200 {
		t.Fatalf("unexpected language/charset: %d/%d", cfg.TranslationID, cfg.CharsetID)
	}
	if !cfg.Is64Bit || cfg.IsARM {
		t.Fatalf("unexpected amd64 flags: Is64Bit=%t IsARM=%t", cfg.Is64Bit, cfg.IsARM)
	}
	if cfg.VerMajor != 1 || cfg.VerMinor != 2 || cfg.VerPatch != 3 || cfg.VerBuild != 0 {
		t.Fatalf("unexpected numeric file version: %d.%d.%d.%d", cfg.VerMajor, cfg.VerMinor, cfg.VerPatch, cfg.VerBuild)
	}
}

func TestBuildConfigARM64(t *testing.T) {
	cfg, err := buildConfig(options{
		productVersion: "0.0.0-dev",
		sourceTime:     "2026-07-21T16:00:00Z",
		goarch:         "arm64",
		output:         "resource_windows_arm64.syso",
		icon:           "flashgate.ico",
	})
	if err != nil {
		t.Fatalf("buildConfig returned error: %v", err)
	}
	if !cfg.Is64Bit || !cfg.IsARM {
		t.Fatalf("unexpected arm64 flags: Is64Bit=%t IsARM=%t", cfg.Is64Bit, cfg.IsARM)
	}
}

func TestBuildConfigRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		opts options
		want string
	}{
		{
			name: "unsupported architecture",
			opts: options{productVersion: "1.2.3", sourceTime: "2026-07-21T16:00:00Z", goarch: "386", output: "x.syso", icon: "x.ico"},
			want: "unsupported GOARCH",
		},
		{
			name: "leading v",
			opts: options{productVersion: "v1.2.3", sourceTime: "2026-07-21T16:00:00Z", goarch: "amd64", output: "x.syso", icon: "x.ico"},
			want: "invalid semantic version",
		},
		{
			name: "non UTC source time",
			opts: options{productVersion: "1.2.3", sourceTime: "2026-07-21T18:00:00+02:00", goarch: "amd64", output: "x.syso", icon: "x.ico"},
			want: "canonical RFC3339 UTC",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildConfig(test.opts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestParseFileVersion(t *testing.T) {
	got, err := parseFileVersion("65535.2.3.0")
	if err != nil {
		t.Fatalf("parseFileVersion returned error: %v", err)
	}
	if got != [4]int{65535, 2, 3, 0} {
		t.Fatalf("unexpected parsed version: %#v", got)
	}

	for _, value := range []string{"1.2.3", "1.2.3.4.5", "1.2.x.0", "65536.0.0.0"} {
		if _, err := parseFileVersion(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}
