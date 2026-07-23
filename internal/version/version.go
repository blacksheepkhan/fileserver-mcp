package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	ProductName        = "FlashGate MCP"
	BinaryName         = "flashgate-mcp"
	FileDescription    = "FlashGate MCP Server"
	CompanyName        = "Thomas Weidner"
	CopyrightHolder    = "Thomas Weidner"
	LicenseName        = "GNU General Public License v3.0"
	ProjectURL         = "https://github.com/thomasweidner/flashgate-mcp"
	Comments           = "Native Model Context Protocol server for controlled local system access."
	OriginalFilename   = "flashgate-mcp.exe"
	InternalName       = "flashgate-mcp"
	defaultVersion     = "0.0.0-dev"
	defaultFileVersion = "0.0.0.0"
	unknownValue       = "unknown"
	maxSourceDateEpoch = int64(253402300799)
)

var (
	version       = defaultVersion
	fileVersion   string
	commit        = unknownValue
	date          = unknownValue
	modified      string
	buildManifest = "unavailable"

	readBuildInfo = debug.ReadBuildInfo
)

// Info contains canonical product, build, and runtime metadata.
type Info struct {
	ProductName   string
	BinaryName    string
	Version       string
	FileVersion   string
	Commit        string
	SourceTime    string
	Modified      bool
	GoVersion     string
	GOOS          string
	GOARCH        string
	PublicArch    string
	Copyright     string
	BuildManifest string
}

// Get returns the canonical metadata embedded in or derivable from the binary.
func Get() Info {
	info := Info{
		ProductName:   ProductName,
		BinaryName:    BinaryName,
		Version:       valueOrDefault(version, defaultVersion),
		FileVersion:   strings.TrimSpace(fileVersion),
		Commit:        valueOrDefault(commit, unknownValue),
		SourceTime:    canonicalTime(valueOrDefault(date, unknownValue)),
		GoVersion:     runtime.Version(),
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		PublicArch:    PublicArchitecture(runtime.GOARCH),
		BuildManifest: buildManifest,
	}

	if buildInfo, ok := readBuildInfo(); ok {
		applyVCSFallback(&info, buildInfo.Settings)
	}

	if info.FileVersion == "" {
		mapped, err := WindowsFileVersion(info.Version)
		if err != nil {
			info.FileVersion = defaultFileVersion
		} else {
			info.FileVersion = mapped
		}
	}

	if parsed, ok := parseBool(modified); ok {
		info.Modified = parsed
	}

	info.Copyright = CopyrightText(info.SourceTime)
	return info
}

// String returns the compact, script-friendly version string.
func (i Info) String() string {
	return fmt.Sprintf("%s %s", i.BinaryName, i.Version)
}

// VerboseString returns the complete human-readable build identity.
func (i Info) VerboseString() string {
	return fmt.Sprintf(
		"Product:      %s\nVersion:      %s\nFile version: %s\nCommit:       %s\nSource time:  %s\nModified:     %t\nGo version:   %s\nPlatform:     %s/%s\nGo target:    %s/%s",
		i.ProductName,
		i.Version,
		i.FileVersion,
		i.Commit,
		i.SourceTime,
		i.Modified,
		i.GoVersion,
		i.GOOS,
		i.PublicArch,
		i.GOOS,
		i.GOARCH,
	)
}

// ShortCommit returns a 12-character commit identifier when possible.
func (i Info) ShortCommit() string {
	if len(i.Commit) <= 12 {
		return i.Commit
	}
	return i.Commit[:12]
}

// PublicArchitecture maps Go architecture names to user-facing identifiers.
func PublicArchitecture(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}

// WindowsFileVersion maps a SemVer product version to Major.Minor.Patch.0.
func WindowsFileVersion(productVersion string) (string, error) {
	major, minor, patch, err := parseSemVer(productVersion)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d.0", major, minor, patch), nil
}

// CopyrightText returns a reproducible copyright string from a canonical UTC source time.
func CopyrightText(canonicalSourceTime string) string {
	parsed, err := time.Parse(time.RFC3339, canonicalSourceTime)
	if err != nil {
		return "Copyright © " + CopyrightHolder
	}
	return fmt.Sprintf("Copyright © %d %s", parsed.UTC().Year(), CopyrightHolder)
}

// SourceTimeFromEpoch validates SOURCE_DATE_EPOCH and returns canonical UTC.
func SourceTimeFromEpoch(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("SOURCE_DATE_EPOCH is empty")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("SOURCE_DATE_EPOCH must contain decimal digits only")
		}
	}

	epoch, err := strconv.ParseInt(value, 10, 64)
	if err != nil || epoch > maxSourceDateEpoch {
		return "", fmt.Errorf("SOURCE_DATE_EPOCH is outside the supported range")
	}
	return time.Unix(epoch, 0).UTC().Format(time.RFC3339), nil
}

func applyVCSFallback(info *Info, settings []debug.BuildSetting) {
	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
	}

	if info.Commit == unknownValue {
		info.Commit = valueOrDefault(values["vcs.revision"], unknownValue)
	}
	if info.SourceTime == unknownValue {
		info.SourceTime = canonicalTime(valueOrDefault(values["vcs.time"], unknownValue))
	}
	if _, explicit := parseBool(modified); !explicit {
		if parsed, ok := parseBool(values["vcs.modified"]); ok {
			info.Modified = parsed
		}
	}
}

func canonicalTime(value string) string {
	if value == unknownValue {
		return unknownValue
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return unknownValue
	}
	return parsed.UTC().Format(time.RFC3339)
}

func parseBool(value string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func valueOrDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func parseSemVer(value string) (uint64, uint64, uint64, error) {
	if strings.TrimSpace(value) != value || value == "" {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q", value)
	}

	withoutBuild, buildMetadata, foundBuild := strings.Cut(value, "+")
	if foundBuild {
		if buildMetadata == "" || strings.Contains(buildMetadata, "+") || !validIdentifierList(buildMetadata, false) {
			return 0, 0, 0, fmt.Errorf("invalid semantic version %q", value)
		}
	}

	core, prerelease, foundPrerelease := strings.Cut(withoutBuild, "-")
	if foundPrerelease {
		if prerelease == "" || !validIdentifierList(prerelease, true) {
			return 0, 0, 0, fmt.Errorf("invalid semantic version %q", value)
		}
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q", value)
	}

	parsed := make([]uint64, 3)
	for index, part := range parts {
		if !validCoreNumber(part) {
			return 0, 0, 0, fmt.Errorf("invalid semantic version %q", value)
		}
		number, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("version component out of Windows range in %q", value)
		}
		parsed[index] = number
	}

	return parsed[0], parsed[1], parsed[2], nil
}

func validCoreNumber(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validIdentifierList(value string, rejectNumericLeadingZero bool) bool {
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for _, character := range identifier {
			if !((character >= '0' && character <= '9') ||
				(character >= 'A' && character <= 'Z') ||
				(character >= 'a' && character <= 'z') ||
				character == '-') {
				return false
			}
			if character < '0' || character > '9' {
				numeric = false
			}
		}
		if rejectNumericLeadingZero && numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}
