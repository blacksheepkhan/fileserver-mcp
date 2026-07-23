package main

import (
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	resourceTypeIcon      = 3
	resourceTypeGroupIcon = 14
	resourceDirectory     = 2
)

type iconFrame struct {
	Width      byte
	Height     byte
	ColorCount byte
	Reserved   byte
	Planes     uint16
	BitCount   uint16
	Length     uint32
	SHA256     string
}

func main() {
	binaryPath := flag.String("binary", "", "Windows PE binary")
	iconPath := flag.String("icon", "", "canonical ICO file")
	flag.Parse()
	if *binaryPath == "" || *iconPath == "" {
		fmt.Fprintln(os.Stderr, "iconverify requires --binary and --icon")
		os.Exit(2)
	}

	expectedData, err := os.ReadFile(*iconPath)
	if err != nil {
		fail(err)
	}
	expected, err := parseICO(expectedData)
	if err != nil {
		fail(err)
	}
	groups, err := extractPEIconGroups(*binaryPath)
	if err != nil {
		fail(err)
	}
	uniqueGroups := make(map[string][]iconFrame)
	for _, group := range groups {
		uniqueGroups[frameIdentityText(group)] = group
	}
	if len(uniqueGroups) != 1 {
		fail(fmt.Errorf("embedded PE contains %d different icon identities", len(uniqueGroups)))
	}
	var actual []iconFrame
	for _, group := range uniqueGroups {
		actual = group
	}
	if err := compareFrames(expected, actual); err != nil {
		fail(err)
	}

	fmt.Println("Status: PASS")
	fmt.Printf("BinaryPath: %s\n", *binaryPath)
	fmt.Printf("IconPath: %s\n", *iconPath)
	fmt.Printf("FrameCount: %d\n", len(actual))
	fmt.Printf("FrameIdentitySHA256: %s\n", frameIdentityHash(actual))
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "iconverify: %v\n", err)
	os.Exit(1)
}

func parseICO(data []byte) ([]iconFrame, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 ||
		binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, errors.New("canonical icon has an invalid ICO header")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || len(data) < 6+count*16 {
		return nil, errors.New("canonical icon has an invalid frame table")
	}
	frames := make([]iconFrame, 0, count)
	for index := 0; index < count; index++ {
		record := data[6+index*16 : 6+(index+1)*16]
		length := binary.LittleEndian.Uint32(record[8:12])
		offset := binary.LittleEndian.Uint32(record[12:16])
		end := uint64(offset) + uint64(length)
		if length == 0 || end > uint64(len(data)) {
			return nil, fmt.Errorf("canonical icon frame %d is out of bounds", index)
		}
		frames = append(
			frames,
			newFrame(record[:12], data[offset:uint32(end)]),
		)
	}
	sortFrames(frames)
	return frames, nil
}

func extractPEIconGroups(binaryPath string) ([][]iconFrame, error) {
	file, err := pe.Open(binaryPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	resourceRVA, err := resourceDirectoryRVA(file)
	if err != nil {
		return nil, err
	}
	resourceSection, rootOffset, err := sectionForRVA(file, resourceRVA)
	if err != nil {
		return nil, err
	}
	resourceData, err := sectionData(resourceSection)
	if err != nil {
		return nil, err
	}

	iconResources, err := readResourceType(
		file,
		resourceData,
		rootOffset,
		resourceTypeIcon,
	)
	if err != nil {
		return nil, err
	}
	groupResources, err := readResourceType(
		file,
		resourceData,
		rootOffset,
		resourceTypeGroupIcon,
	)
	if err != nil {
		return nil, err
	}
	if len(groupResources) == 0 {
		return nil, errors.New("embedded PE contains no icon group")
	}

	return parseIconGroups(iconResources, groupResources)
}

func parseIconGroups(
	iconResources map[uint16][]byte,
	groupResources map[uint16][]byte,
) ([][]iconFrame, error) {
	groups := make([][]iconFrame, 0, len(groupResources))
	allReferencedIDs := make(map[uint16]struct{})
	for _, group := range groupResources {
		if len(group) < 6 || binary.LittleEndian.Uint16(group[0:2]) != 0 ||
			binary.LittleEndian.Uint16(group[2:4]) != 1 {
			return nil, errors.New("embedded group icon header is invalid")
		}
		count := int(binary.LittleEndian.Uint16(group[4:6]))
		if count == 0 || len(group) != 6+count*14 {
			return nil, errors.New("embedded group icon table is invalid")
		}

		frames := make([]iconFrame, 0, count)
		seenIDs := make(map[uint16]struct{}, count)
		for index := 0; index < count; index++ {
			record := group[6+index*14 : 6+(index+1)*14]
			resourceID := binary.LittleEndian.Uint16(record[12:14])
			if _, exists := seenIDs[resourceID]; exists {
				return nil, fmt.Errorf("duplicate icon resource ID %d", resourceID)
			}
			seenIDs[resourceID] = struct{}{}
			allReferencedIDs[resourceID] = struct{}{}
			payload, exists := iconResources[resourceID]
			if !exists {
				return nil, fmt.Errorf("icon resource ID %d is missing", resourceID)
			}
			expectedLength := binary.LittleEndian.Uint32(record[8:12])
			if uint32(len(payload)) != expectedLength {
				return nil, fmt.Errorf("icon resource ID %d has an unexpected length", resourceID)
			}
			frames = append(frames, newFrame(record[:12], payload))
		}
		sortFrames(frames)
		groups = append(groups, frames)
	}
	if len(iconResources) != len(allReferencedIDs) {
		return nil, errors.New("unreferenced embedded icon resources are present")
	}
	return groups, nil
}

func resourceDirectoryRVA(file *pe.File) (uint32, error) {
	switch optional := file.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		if len(optional.DataDirectory) <= resourceDirectory {
			return 0, errors.New("PE resource data directory is missing")
		}
		return optional.DataDirectory[resourceDirectory].VirtualAddress, nil
	case *pe.OptionalHeader32:
		if len(optional.DataDirectory) <= resourceDirectory {
			return 0, errors.New("PE resource data directory is missing")
		}
		return optional.DataDirectory[resourceDirectory].VirtualAddress, nil
	default:
		return 0, errors.New("unsupported PE optional header")
	}
}

func sectionForRVA(file *pe.File, rva uint32) (*pe.Section, uint32, error) {
	for _, section := range file.Sections {
		size := section.Size
		if section.VirtualSize > size {
			size = section.VirtualSize
		}
		if rva >= section.VirtualAddress && rva-section.VirtualAddress < size {
			return section, rva - section.VirtualAddress, nil
		}
	}
	return nil, 0, fmt.Errorf("RVA 0x%x is outside PE sections", rva)
}

func readResourceType(
	file *pe.File,
	data []byte,
	rootOffset uint32,
	resourceType uint32,
) (map[uint16][]byte, error) {
	typeDirectory, err := findDirectoryID(data, rootOffset, rootOffset, resourceType)
	if err != nil {
		return nil, err
	}
	nameEntries, err := directoryEntries(data, rootOffset, typeDirectory)
	if err != nil {
		return nil, err
	}
	result := make(map[uint16][]byte)
	for _, nameEntry := range nameEntries {
		if nameEntry.name&0x80000000 != 0 || nameEntry.offset&0x80000000 == 0 {
			return nil, errors.New("named or non-directory resource entry rejected")
		}
		resourceID := uint16(nameEntry.name)
		languageDirectory := rootOffset + (nameEntry.offset & 0x7fffffff)
		languageEntries, err := directoryEntries(data, rootOffset, languageDirectory)
		if err != nil {
			return nil, err
		}
		if len(languageEntries) != 1 {
			return nil, fmt.Errorf("resource ID %d has %d language entries", resourceID, len(languageEntries))
		}
		language := languageEntries[0]
		if language.offset&0x80000000 != 0 {
			return nil, errors.New("unexpected fourth resource-directory level")
		}
		dataEntryOffset := rootOffset + language.offset
		if uint64(dataEntryOffset)+16 > uint64(len(data)) {
			return nil, errors.New("resource data entry is out of bounds")
		}
		resourceRVA := binary.LittleEndian.Uint32(data[dataEntryOffset : dataEntryOffset+4])
		resourceLength := binary.LittleEndian.Uint32(data[dataEntryOffset+4 : dataEntryOffset+8])
		section, offset, err := sectionForRVA(file, resourceRVA)
		if err != nil {
			return nil, err
		}
		payloadSectionData, err := sectionData(section)
		if err != nil {
			return nil, err
		}
		end := uint64(offset) + uint64(resourceLength)
		if resourceLength == 0 || end > uint64(len(payloadSectionData)) {
			return nil, errors.New("resource payload is out of bounds")
		}
		payload := append([]byte(nil), payloadSectionData[offset:uint32(end)]...)
		if _, exists := result[resourceID]; exists {
			return nil, fmt.Errorf("duplicate resource ID %d", resourceID)
		}
		result[resourceID] = payload
	}
	return result, nil
}

func sectionData(section *pe.Section) ([]byte, error) {
	if section.ReaderAt == nil {
		return section.Data()
	}
	data := make([]byte, section.Size)
	reader := io.NewSectionReader(section.ReaderAt, 0, int64(section.Size))
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data, nil
}

type resourceEntry struct {
	name   uint32
	offset uint32
}

func findDirectoryID(
	data []byte,
	rootOffset uint32,
	directoryOffset uint32,
	id uint32,
) (uint32, error) {
	entries, err := directoryEntries(data, rootOffset, directoryOffset)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if entry.name == id {
			if entry.offset&0x80000000 == 0 {
				return 0, fmt.Errorf("resource type %d is not a directory", id)
			}
			return rootOffset + (entry.offset & 0x7fffffff), nil
		}
	}
	return 0, fmt.Errorf("resource type %d is missing", id)
}

func directoryEntries(
	data []byte,
	rootOffset uint32,
	directoryOffset uint32,
) ([]resourceEntry, error) {
	if uint64(directoryOffset)+16 > uint64(len(data)) {
		return nil, errors.New("resource directory is out of bounds")
	}
	named := binary.LittleEndian.Uint16(data[directoryOffset+12 : directoryOffset+14])
	ids := binary.LittleEndian.Uint16(data[directoryOffset+14 : directoryOffset+16])
	count := int(named) + int(ids)
	end := uint64(directoryOffset) + 16 + uint64(count)*8
	if count == 0 || end > uint64(len(data)) {
		return nil, errors.New("resource directory entries are invalid")
	}
	entries := make([]resourceEntry, 0, count)
	for index := 0; index < count; index++ {
		offset := directoryOffset + 16 + uint32(index)*8
		entries = append(
			entries,
			resourceEntry{
				name:   binary.LittleEndian.Uint32(data[offset : offset+4]),
				offset: binary.LittleEndian.Uint32(data[offset+4 : offset+8]),
			},
		)
	}
	_ = rootOffset
	return entries, nil
}

func newFrame(record []byte, payload []byte) iconFrame {
	digest := sha256.Sum256(payload)
	return iconFrame{
		Width:      record[0],
		Height:     record[1],
		ColorCount: record[2],
		Reserved:   record[3],
		Planes:     binary.LittleEndian.Uint16(record[4:6]),
		BitCount:   binary.LittleEndian.Uint16(record[6:8]),
		Length:     binary.LittleEndian.Uint32(record[8:12]),
		SHA256:     hex.EncodeToString(digest[:]),
	}
}

func sortFrames(frames []iconFrame) {
	sort.Slice(frames, func(left, right int) bool {
		return frameIdentity(frames[left]) < frameIdentity(frames[right])
	})
}

func compareFrames(expected, actual []iconFrame) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("icon frame count mismatch: expected %d, found %d", len(expected), len(actual))
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return fmt.Errorf("icon frame identity mismatch at normalized index %d", index)
		}
	}
	return nil
}

func frameIdentity(frame iconFrame) string {
	return fmt.Sprintf(
		"%03d:%03d:%03d:%03d:%05d:%05d:%010d:%s",
		frame.Width,
		frame.Height,
		frame.ColorCount,
		frame.Reserved,
		frame.Planes,
		frame.BitCount,
		frame.Length,
		frame.SHA256,
	)
}

func frameIdentityHash(frames []iconFrame) string {
	digest := sha256.Sum256([]byte(frameIdentityText(frames)))
	return hex.EncodeToString(digest[:])
}

func frameIdentityText(frames []iconFrame) string {
	var builder strings.Builder
	for _, frame := range frames {
		builder.WriteString(frameIdentity(frame))
		builder.WriteByte('\n')
	}
	return builder.String()
}
