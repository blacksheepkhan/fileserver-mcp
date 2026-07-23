package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestGetReturnsDeterministicDevelopmentDefaults(t *testing.T) {
	restore := setBuildValuesForTest(t, defaultVersion, "", unknownValue, unknownValue, "")
	defer restore()

	originalReadBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	defer func() { readBuildInfo = originalReadBuildInfo }()

	info := Get()

	if info.ProductName != ProductName {
		t.Fatalf("expected product name %q, got %q", ProductName, info.ProductName)
	}
	if info.BinaryName != BinaryName {
		t.Fatalf("expected binary name %q, got %q", BinaryName, info.BinaryName)
	}
	if info.Version != "0.0.0-dev" {
		t.Fatalf("expected default version 0.0.0-dev, got %q", info.Version)
	}
	if info.FileVersion != "0.0.0.0" {
		t.Fatalf("expected default file version 0.0.0.0, got %q", info.FileVersion)
	}
	if info.Commit != "unknown" {
		t.Fatalf("expected default commit unknown, got %q", info.Commit)
	}
	if info.SourceTime != "unknown" {
		t.Fatalf("expected default source time unknown, got %q", info.SourceTime)
	}
	if info.Modified {
		t.Fatal("expected default modified state false")
	}
	if info.GoVersion == "" || info.GOOS == "" || info.GOARCH == "" || info.PublicArch == "" {
		t.Fatalf("expected runtime metadata, got %+v", info)
	}
	if info.Copyright != "Copyright © Thomas Weidner" {
		t.Fatalf("unexpected fallback copyright %q", info.Copyright)
	}
}

func TestGetUsesVCSFallbackAndCanonicalUTC(t *testing.T) {
	restore := setBuildValuesForTest(t, defaultVersion, "", unknownValue, unknownValue, "")
	defer restore()

	originalReadBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: "2026-07-21T18:30:00+02:00"},
			{Key: "vcs.modified", Value: "true"},
		}}, true
	}
	defer func() { readBuildInfo = originalReadBuildInfo }()

	info := Get()

	if info.Commit != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("unexpected commit %q", info.Commit)
	}
	if info.SourceTime != "2026-07-21T16:30:00Z" {
		t.Fatalf("expected canonical UTC time, got %q", info.SourceTime)
	}
	if !info.Modified {
		t.Fatal("expected modified state from VCS metadata")
	}
	if info.Copyright != "Copyright © 2026 Thomas Weidner" {
		t.Fatalf("unexpected copyright %q", info.Copyright)
	}
}

func TestExplicitBuildValuesTakePrecedence(t *testing.T) {
	restore := setBuildValuesForTest(
		t,
		"1.2.3-rc.1",
		"1.2.3.0",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"2026-01-02T03:04:05Z",
		"false",
	)
	defer restore()

	originalReadBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "ffffffffffffffffffffffffffffffffffffffff"},
			{Key: "vcs.time", Value: "2025-01-01T00:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		}}, true
	}
	defer func() { readBuildInfo = originalReadBuildInfo }()

	info := Get()

	if info.Version != "1.2.3-rc.1" || info.FileVersion != "1.2.3.0" {
		t.Fatalf("unexpected versions: %+v", info)
	}
	if info.Commit != "abcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Fatalf("explicit commit was not preserved: %q", info.Commit)
	}
	if info.SourceTime != "2026-01-02T03:04:05Z" {
		t.Fatalf("explicit source time was not preserved: %q", info.SourceTime)
	}
	if info.Modified {
		t.Fatal("explicit modified=false was not preserved")
	}
}

func TestWindowsFileVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{name: "stable", version: "1.2.3", want: "1.2.3.0"},
		{name: "prerelease", version: "1.2.3-rc.1", want: "1.2.3.0"},
		{name: "build metadata", version: "1.2.3+build.7", want: "1.2.3.0"},
		{name: "development", version: "0.0.0-dev", want: "0.0.0.0"},
		{name: "leading v", version: "v1.2.3", wantErr: true},
		{name: "leading zero", version: "01.2.3", wantErr: true},
		{name: "numeric prerelease leading zero", version: "1.2.3-01", wantErr: true},
		{name: "missing patch", version: "1.2", wantErr: true},
		{name: "overflow", version: "65536.0.0", wantErr: true},
		{name: "empty", version: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := WindowsFileVersion(test.version)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", test.version, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", test.version, err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestPublicArchitecture(t *testing.T) {
	tests := map[string]string{
		"amd64":   "x64",
		"arm64":   "arm64",
		"riscv64": "riscv64",
	}

	for input, expected := range tests {
		if got := PublicArchitecture(input); got != expected {
			t.Fatalf("PublicArchitecture(%q): expected %q, got %q", input, expected, got)
		}
	}
}

func TestInfoStrings(t *testing.T) {
	info := Info{
		ProductName: "FlashGate MCP",
		BinaryName:  "flashgate-mcp",
		Version:     "1.2.3",
		FileVersion: "1.2.3.0",
		Commit:      "0123456789abcdef0123456789abcdef01234567",
		SourceTime:  "2026-07-21T16:30:00Z",
		Modified:    false,
		GoVersion:   "go1.26.5",
		GOOS:        "windows",
		GOARCH:      "amd64",
		PublicArch:  "x64",
	}

	if got := info.String(); got != "flashgate-mcp 1.2.3" {
		t.Fatalf("unexpected compact string %q", got)
	}
	if got := info.ShortCommit(); got != "0123456789ab" {
		t.Fatalf("unexpected short commit %q", got)
	}

	verbose := info.VerboseString()
	for _, expected := range []string{
		"Product:      FlashGate MCP",
		"Version:      1.2.3",
		"File version: 1.2.3.0",
		"Commit:       0123456789abcdef0123456789abcdef01234567",
		"Source time:  2026-07-21T16:30:00Z",
		"Modified:     false",
		"Go version:   go1.26.5",
		"Platform:     windows/x64",
		"Go target:    windows/amd64",
	} {
		if !strings.Contains(verbose, expected) {
			t.Fatalf("expected verbose output to contain %q, got %q", expected, verbose)
		}
	}
}

func setBuildValuesForTest(t *testing.T, testVersion, testFileVersion, testCommit, testDate, testModified string) func() {
	t.Helper()
	oldVersion := version
	oldFileVersion := fileVersion
	oldCommit := commit
	oldDate := date
	oldModified := modified

	version = testVersion
	fileVersion = testFileVersion
	commit = testCommit
	date = testDate
	modified = testModified

	return func() {
		version = oldVersion
		fileVersion = oldFileVersion
		commit = oldCommit
		date = oldDate
		modified = oldModified
	}
}
