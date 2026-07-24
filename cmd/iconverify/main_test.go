package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"testing"
)

func TestParseICOAndCompareFrames(t *testing.T) {
	t.Parallel()

	canonical := makeICO([]byte("canonical-frame"))
	expected, err := parseICO(canonical)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := parseICO(append([]byte(nil), canonical...))
	if err != nil {
		t.Fatal(err)
	}
	if err := compareFrames(expected, actual); err != nil {
		t.Fatal(err)
	}

	manipulated := append([]byte(nil), canonical...)
	manipulated[len(manipulated)-1] ^= 0xff
	other, err := parseICO(manipulated)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareFrames(expected, other); err == nil {
		t.Fatal("manipulated icon frame unexpectedly matched")
	}
}

func TestParseICORejectsInvalidBounds(t *testing.T) {
	t.Parallel()
	icon := makeICO([]byte("frame"))
	binary.LittleEndian.PutUint32(icon[14:18], uint32(len(icon)+1))
	if _, err := parseICO(icon); err == nil {
		t.Fatal("out-of-bounds icon frame unexpectedly passed")
	}
}

func TestParseIconGroups(t *testing.T) {
	t.Parallel()
	payload := []byte("canonical-frame")
	ico := makeICO(payload)
	expected, err := parseICO(ico)
	if err != nil {
		t.Fatal(err)
	}

	group := make([]byte, 20)
	binary.LittleEndian.PutUint16(group[2:4], 1)
	binary.LittleEndian.PutUint16(group[4:6], 1)
	copy(group[6:18], ico[6:18])
	binary.LittleEndian.PutUint16(group[18:20], 1)
	groups, err := parseIconGroups(
		map[uint16][]byte{1: payload},
		map[uint16][]byte{1: group, 2: group},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected two language/group identities, got %d", len(groups))
	}
	for _, actual := range groups {
		if err := compareFrames(expected, actual); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := parseIconGroups(
		map[uint16][]byte{1: payload, 2: []byte("unused")},
		map[uint16][]byte{1: group},
	); err == nil {
		t.Fatal("unreferenced icon resource unexpectedly passed")
	}
	if frameIdentityHash(expected) == "" ||
		frameIdentityText(expected) == "" ||
		frameIdentity(expected[0]) == "" {
		t.Fatal("normalized frame identity is empty")
	}
}

func TestResourceDirectoryHelpers(t *testing.T) {
	t.Parallel()
	data := make([]byte, 64)
	binary.LittleEndian.PutUint16(data[8+14:8+16], 1)
	binary.LittleEndian.PutUint32(data[24:28], 3)
	binary.LittleEndian.PutUint32(data[28:32], 0x80000020)

	entries, err := directoryEntries(data, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].name != 3 {
		t.Fatalf("unexpected resource entries: %+v", entries)
	}
	offset, err := findDirectoryID(data, 8, 8, 3)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 40 {
		t.Fatalf("unexpected directory offset %d", offset)
	}
	if _, err := findDirectoryID(data, 8, 8, 14); err == nil {
		t.Fatal("missing resource ID unexpectedly passed")
	}
	if _, err := directoryEntries(data[:10], 0, 0); err == nil {
		t.Fatal("out-of-bounds resource directory unexpectedly passed")
	}

	file64 := &pe.File{
		OptionalHeader: &pe.OptionalHeader64{
			DataDirectory: [16]pe.DataDirectory{
				resourceDirectory: {VirtualAddress: 0x1234},
			},
		},
	}
	rva, err := resourceDirectoryRVA(file64)
	if err != nil || rva != 0x1234 {
		t.Fatalf("unexpected resource RVA 0x%x: %v", rva, err)
	}
	file32 := &pe.File{
		OptionalHeader: &pe.OptionalHeader32{
			DataDirectory: [16]pe.DataDirectory{
				resourceDirectory: {VirtualAddress: 0x5678},
			},
		},
	}
	rva, err = resourceDirectoryRVA(file32)
	if err != nil || rva != 0x5678 {
		t.Fatalf("unexpected resource RVA 0x%x: %v", rva, err)
	}

	section := &pe.Section{
		SectionHeader: pe.SectionHeader{
			VirtualAddress: 0x1000,
			VirtualSize:    0x200,
			Size:           0x100,
		},
		ReaderAt: bytes.NewReader(make([]byte, 0x100)),
	}
	resolved, relative, err := sectionForRVA(
		&pe.File{Sections: []*pe.Section{section}},
		0x1080,
	)
	if err != nil || resolved != section || relative != 0x80 {
		t.Fatalf("unexpected section resolution: %v, 0x%x, %v", resolved, relative, err)
	}
	if _, _, err := sectionForRVA(
		&pe.File{Sections: []*pe.Section{section}},
		0x2000,
	); err == nil {
		t.Fatal("out-of-section RVA unexpectedly passed")
	}
}

func TestReadResourceType(t *testing.T) {
	t.Parallel()
	data := make([]byte, 128)
	writeDirectoryEntry := func(directoryOffset int, name, offset uint32) {
		binary.LittleEndian.PutUint16(
			data[directoryOffset+14:directoryOffset+16],
			1,
		)
		binary.LittleEndian.PutUint32(
			data[directoryOffset+16:directoryOffset+20],
			name,
		)
		binary.LittleEndian.PutUint32(
			data[directoryOffset+20:directoryOffset+24],
			offset,
		)
	}
	writeDirectoryEntry(0, resourceTypeIcon, 0x80000018)
	writeDirectoryEntry(24, 1, 0x80000030)
	writeDirectoryEntry(48, 1033, 72)
	binary.LittleEndian.PutUint32(data[72:76], 0x1000+100)
	binary.LittleEndian.PutUint32(data[76:80], 3)
	copy(data[100:103], []byte("ico"))

	section := &pe.Section{
		SectionHeader: pe.SectionHeader{
			VirtualAddress: 0x1000,
			VirtualSize:    uint32(len(data)),
			Size:           uint32(len(data)),
		},
		ReaderAt: bytes.NewReader(data),
	}
	resources, err := readResourceType(
		&pe.File{Sections: []*pe.Section{section}},
		data,
		0,
		resourceTypeIcon,
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(resources[1]) != "ico" {
		t.Fatalf("unexpected resource payload %q", resources[1])
	}
}

func makeICO(payload []byte) []byte {
	result := make([]byte, 22+len(payload))
	binary.LittleEndian.PutUint16(result[2:4], 1)
	binary.LittleEndian.PutUint16(result[4:6], 1)
	result[6] = 16
	result[7] = 16
	binary.LittleEndian.PutUint16(result[10:12], 1)
	binary.LittleEndian.PutUint16(result[12:14], 32)
	binary.LittleEndian.PutUint32(result[14:18], uint32(len(payload)))
	binary.LittleEndian.PutUint32(result[18:22], 22)
	copy(result[22:], payload)
	return result
}
