package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/josephspurrier/goversioninfo"
	"github.com/thomasweidner/flashgate-mcp/internal/version"
)

type options struct {
	productVersion string
	sourceTime     string
	goarch         string
	output         string
	icon           string
}

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("flashgate-versioninfo", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var opts options
	flags.StringVar(&opts.productVersion, "version", "", "semantic product version without a leading v")
	flags.StringVar(&opts.sourceTime, "source-time", "", "canonical RFC3339 source time in UTC")
	flags.StringVar(&opts.goarch, "goarch", "", "target Go architecture: amd64 or arm64")
	flags.StringVar(&opts.output, "output", "", "target architecture-specific .syso file")
	flags.StringVar(&opts.icon, "icon", "", "path to the committed FlashGate .ico file")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}

	cfg, err := buildConfig(opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(opts.output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if _, err := os.Stat(opts.icon); err != nil {
		return fmt.Errorf("read icon %q: %w", opts.icon, err)
	}
	if _, err := os.Stat(opts.output); err == nil {
		return fmt.Errorf("refusing to overwrite existing resource %q", opts.output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output %q: %w", opts.output, err)
	}

	if err := goversioninfo.RunCLI(cfg); err != nil {
		return fmt.Errorf("generate Windows resource: %w", err)
	}
	return nil
}

func buildConfig(opts options) (goversioninfo.CLIConfig, error) {
	if opts.productVersion == "" {
		return goversioninfo.CLIConfig{}, errors.New("-version is required")
	}
	if opts.sourceTime == "" {
		return goversioninfo.CLIConfig{}, errors.New("-source-time is required")
	}
	if opts.output == "" {
		return goversioninfo.CLIConfig{}, errors.New("-output is required")
	}
	if opts.icon == "" {
		return goversioninfo.CLIConfig{}, errors.New("-icon is required")
	}
	if opts.goarch != "amd64" && opts.goarch != "arm64" {
		return goversioninfo.CLIConfig{}, fmt.Errorf("unsupported GOARCH %q", opts.goarch)
	}

	sourceTime, err := time.Parse(time.RFC3339, opts.sourceTime)
	if err != nil {
		return goversioninfo.CLIConfig{}, fmt.Errorf("invalid source time %q: %w", opts.sourceTime, err)
	}
	if sourceTime.Format(time.RFC3339) != opts.sourceTime || sourceTime.Location() != time.UTC {
		return goversioninfo.CLIConfig{}, fmt.Errorf("source time must be canonical RFC3339 UTC with Z: %q", opts.sourceTime)
	}

	fileVersion, err := version.WindowsFileVersion(opts.productVersion)
	if err != nil {
		return goversioninfo.CLIConfig{}, err
	}
	parts, err := parseFileVersion(fileVersion)
	if err != nil {
		return goversioninfo.CLIConfig{}, err
	}

	cfg := goversioninfo.NewCLIConfig()
	cfg.SkipVersionInfo = true
	cfg.OutputFile = opts.output
	cfg.IconPath = opts.icon
	cfg.ApplicationIconPath = opts.icon
	cfg.Comment = version.Comments
	cfg.CompanyName = version.CompanyName
	cfg.Description = version.FileDescription
	cfg.FileVersion = fileVersion
	cfg.InternalName = version.InternalName
	cfg.Copyright = version.CopyrightText(opts.sourceTime)
	cfg.OriginalName = version.OriginalFilename
	cfg.ProductName = version.ProductName
	cfg.ProductVersion = opts.productVersion
	cfg.TranslationID = 0x0409
	cfg.CharsetID = 1200
	cfg.Is64Bit = true
	cfg.IsARM = opts.goarch == "arm64"

	cfg.VerMajor = parts[0]
	cfg.VerMinor = parts[1]
	cfg.VerPatch = parts[2]
	cfg.VerBuild = parts[3]
	cfg.ProductVerMajor = parts[0]
	cfg.ProductVerMinor = parts[1]
	cfg.ProductVerPatch = parts[2]
	cfg.ProductVerBuild = parts[3]

	return cfg, nil
}

func parseFileVersion(value string) ([4]int, error) {
	var result [4]int
	parts := strings.Split(value, ".")
	if len(parts) != len(result) {
		return result, fmt.Errorf("invalid Windows file version %q", value)
	}
	for index, part := range parts {
		number, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return result, fmt.Errorf("invalid Windows file version %q: %w", value, err)
		}
		result[index] = int(number)
	}
	return result, nil
}
