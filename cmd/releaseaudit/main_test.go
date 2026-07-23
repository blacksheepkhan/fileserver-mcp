package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTarGZRejectsUnsafeEntryTypesAndDuplicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		firstType  byte
		secondName string
		secondType byte
	}{
		{name: "symlink", firstType: tar.TypeSymlink},
		{name: "hardlink", firstType: tar.TypeLink},
		{name: "fifo", firstType: tar.TypeFifo},
		{name: "device", firstType: tar.TypeChar},
		{name: "socket", firstType: byte('s')},
		{
			name:       "duplicate",
			firstType:  tar.TypeReg,
			secondName: "root/file",
			secondType: tar.TypeReg,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			archivePath := filepath.Join(t.TempDir(), "fixture.tar.gz")
			writeTarFixture(
				t,
				archivePath,
				test.firstType,
				test.secondName,
				test.secondType,
			)
			if _, err := readArchive(archivePath); err == nil {
				t.Fatal("malicious TAR fixture unexpectedly passed")
			}
		})
	}
}

func TestNormalizeArchivePathRejectsTraversal(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"../escape",
		"/absolute",
		`..\escape`,
		"root/../escape",
		"",
	} {
		if _, err := normalizeArchivePath(value); err == nil {
			t.Fatalf("unsafe path %q unexpectedly passed", value)
		}
	}
}

func TestReleaseAuditCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	archiveA := filepath.Join(root, "a.zip")
	archiveB := filepath.Join(root, "b.zip")
	writeZIPFixture(t, archiveA, []byte("binary"))
	data, err := os.ReadFile(archiveA)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archiveB, data, 0o600); err != nil {
		t.Fatal(err)
	}
	checksumA := filepath.Join(root, "a.sha256")
	checksumB := filepath.Join(root, "b.sha256")
	for _, checksumPath := range []string{checksumA, checksumB} {
		if err := os.WriteFile(checksumPath, []byte("same checksum\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	inventoryPath := filepath.Join(root, "inventory.json")
	if err := runInventory([]string{
		"--artifact", archiveA,
		"--report", inventoryPath,
	}); err != nil {
		t.Fatal(err)
	}
	var inventory inventoryReport
	readJSONFixture(t, inventoryPath, &inventory)
	if inventory.Status != "PASS" || len(inventory.Entries) != 2 {
		t.Fatalf("unexpected inventory report: %+v", inventory)
	}

	comparisonPath := filepath.Join(root, "comparison.json")
	if err := runCompare([]string{
		"--artifact-a", archiveA,
		"--checksum-a", checksumA,
		"--artifact-b", archiveB,
		"--checksum-b", checksumB,
		"--binary-suffix", "/flashgate-mcp",
		"--report", comparisonPath,
	}); err != nil {
		t.Fatal(err)
	}
	var comparison comparisonReport
	readJSONFixture(t, comparisonPath, &comparison)
	if comparison.Status != "PASS" || comparison.BinarySHA256 == "" {
		t.Fatalf("unexpected comparison report: %+v", comparison)
	}

	scanPath := filepath.Join(root, "scan.json")
	if err := runScan([]string{
		"--artifact", archiveA,
		"--checksum", checksumA,
		"--report", scanPath,
		"--forbidden", "not-present",
	}); err != nil {
		t.Fatal(err)
	}
	var scan leakReport
	readJSONFixture(t, scanPath, &scan)
	if scan.Status != "PASS" || scan.ScannedEntries != 2 {
		t.Fatalf("unexpected scan report: %+v", scan)
	}

	fileScanPath := filepath.Join(root, "file-scan.json")
	if err := runScanFile([]string{
		"--file", archiveA,
		"--report", fileScanPath,
		"--forbidden", "not-present",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLeakRulesAndStringHelpers(t *testing.T) {
	t.Parallel()

	var forbidden stringList
	if err := forbidden.Set("host-value"); err != nil {
		t.Fatal(err)
	}
	if forbidden.String() != "host-value" {
		t.Fatalf("unexpected string-list output %q", forbidden.String())
	}
	if err := forbidden.Set(""); err == nil {
		t.Fatal("empty forbidden value unexpectedly passed")
	}

	token := strings.Join(
		[]string{"github", "pat", strings.Repeat("A", 24)},
		"_",
	)
	privateKeyShape := strings.Join([]string{
		"-----BEGIN ", "PRIVATE KEY-----\n",
		strings.Repeat("A", 40),
		"\n-----END",
	}, "")
	content := []byte(strings.Join([]string{
		`C:\Users\example\AppData\Local\Temp\file`,
		"/home/example/repository",
		token,
		"AK" + "IA" + strings.Repeat("A", 16),
		"https://user:password@example.invalid",
		privateKeyShape,
		"host-value",
	}, "\n"))
	report := leakReport{}
	scanBytes("fixture", content, forbidden, &report)
	if len(report.Findings) != 7 {
		t.Fatalf("expected 7 leak findings, got %+v", report.Findings)
	}

	utf16Data := make([]byte, 0, len("host-value")*2)
	for _, character := range "host-value" {
		utf16Data = append(utf16Data, byte(character), 0)
	}
	if !strings.Contains(printableUTF16LE(utf16Data), "host-value") {
		t.Fatal("UTF-16LE printable extraction failed")
	}
	if !strings.Contains(printableASCII([]byte("xx\x00printable-value\x00")), "printable-value") {
		t.Fatal("ASCII printable extraction failed")
	}
}

func TestCompareDetectsDifferentArchives(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	archiveA := filepath.Join(root, "a.zip")
	archiveB := filepath.Join(root, "b.zip")
	writeZIPFixture(t, archiveA, []byte("first"))
	writeZIPFixture(t, archiveB, []byte("second"))
	checksumA := filepath.Join(root, "a.sha256")
	checksumB := filepath.Join(root, "b.sha256")
	if err := os.WriteFile(checksumA, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checksumB, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(root, "comparison.json")
	if err := runCompare([]string{
		"--artifact-a", archiveA,
		"--checksum-a", checksumA,
		"--artifact-b", archiveB,
		"--checksum-b", checksumB,
		"--binary-suffix", "/flashgate-mcp",
		"--report", reportPath,
	}); err == nil {
		t.Fatal("different archives unexpectedly compared equal")
	}
}

func writeTarFixture(
	t *testing.T,
	archivePath string,
	firstType byte,
	secondName string,
	secondType byte,
) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	write := func(name string, entryType byte) {
		header := &tar.Header{
			Name:     name,
			Typeflag: entryType,
			Mode:     0o644,
			Size:     0,
			Linkname: "target",
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	write("root/file", firstType)
	if secondName != "" {
		write(secondName, secondType)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZIPFixture(t *testing.T, archivePath string, binary []byte) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	directory := &zip.FileHeader{Name: "root/"}
	directory.SetMode(os.ModeDir | 0o755)
	if _, err := writer.CreateHeader(directory); err != nil {
		t.Fatal(err)
	}
	header := &zip.FileHeader{Name: "root/flashgate-mcp"}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readJSONFixture(t *testing.T, filePath string, value any) {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}
