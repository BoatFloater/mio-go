package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const offsetV1 = 0x0
const offsetV2 = 0x3000
const offsetVersionSearch = 0x14

const offsetBrand = 0x35
const offsetName = 0x1C
const offsetAuthor = 0x48
const offsetCode = 0xCF
const offsetItemNumber = 0xD4
const offsetItemRevision = 0xD

const lenVersionStr = 4
const lenBrand = 9
const lenName = 0x18
const lenAuthor = 0x18

type medium struct {
	dataName  string
	dataStart uint32
	dataIndex uint32
	dataSize  uint32
}

var debug bool
var overwrite bool
var outdir string
var format string

func main() {
	mediumMicrogame := &medium{
		dataName:  "microgame",
		dataStart: 0x120000,
		dataIndex: 0x6E0,
		dataSize:  0x10000,
	}

	mediumRecord := &medium{
		dataName:  "record",
		dataStart: 0x920000,
		dataIndex: 0x796,
		dataSize:  0x2000,
	}

	mediumComic := &medium{
		dataName:  "comic",
		dataStart: 0xA20000,
		dataIndex: 0x84C,
		dataSize:  0x3800,
	}

	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.BoolVar(&overwrite, "overwrite", true, "Overwrite existing files")
	flag.StringVar(&outdir, "outdir", "out", "Output directory")
	flag.StringVar(&format, "format", "{code} - {name}", "Filename format; available variables are {name}, {brand}, {author}, {code}")
	flag.Parse()

	fileName := flag.Arg(0)

	if len(fileName) == 0 {
		fmt.Printf("Usage: %s [flags] savefile\nFlags:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		return
	}

	file, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("error opening file %v\n", fileName)
		return
	}

	checkIsSav := []byte{0x0E, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x53, 0x4D, 0x49, 0x4F, 0x5F, 0x53}
	testIsSav := make([]byte, len(checkIsSav))
	_, _ = file.ReadAt(testIsSav, 0) //todo handle

	if !compareBytes(checkIsSav, testIsSav) {
		fmt.Printf("Error: file does not appear to be a WarioWare DIY SAV!\n")
		return
	}

	ver1 := verSearch(file, offsetV1)
	ver2 := verSearch(file, offsetV2)
	if debug {
		fmt.Printf("ver1: %v\nver2: %v\n", ver1, ver2)
	}

	saveOffset := offsetV1
	if ver2 > ver1 {
		saveOffset = offsetV2
	}

	microgames := readShelf(file, saveOffset, mediumMicrogame)
	for i := 0; i < len(microgames); i++ {
		dumpItem(file, microgames[i], mediumMicrogame)
	}
	record := readShelf(file, saveOffset, mediumRecord)
	for i := 0; i < len(record); i++ {
		dumpItem(file, record[i], mediumRecord)
	}
	comic := readShelf(file, saveOffset, mediumComic)
	for i := 0; i < len(comic); i++ {
		dumpItem(file, comic[i], mediumComic)
	}

}

func verSearch(file *os.File, offset int) int {
	start := offset + offsetVersionSearch
	b := make([]byte, lenVersionStr)
	_, _ = file.ReadAt(b, int64(start)) //todo handle
	result := 0
	for i := 0; i < lenVersionStr; i++ {
		result += int(b[i])
	}
	return result
}

func readShelf(file *os.File, saveOffset int, medium *medium) []byte {
	if debug {
		fmt.Printf("checking %vs...\n", medium.dataName)
	}
	shelf := make([]byte, 0xB4)
	var foundItems []byte
	_, _ = file.ReadAt(shelf, int64(saveOffset)+int64(medium.dataIndex))
	for i := 0; i < len(shelf); i += 2 {
		if debug {
			fmt.Printf("%02X ", shelf[i])
		}
		if shelf[i] == 0x0 {
			continue
		}
		foundItems = append(foundItems, shelf[i])
	}
	if debug {
		fmt.Println()
	}
	fmt.Printf("Found %v %vs\n", len(foundItems), medium.dataName)
	return foundItems
}

func dumpItem(file *os.File, itemOffset byte, medium *medium) {
	dirPath := fmt.Sprintf("%v/%v", outdir, medium.dataName)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.MkdirAll(dirPath, os.ModePerm)
	}

	brand := readBrand(file, itemOffset, medium)
	name := readName(file, itemOffset, medium)
	author := readAuthor(file, itemOffset, medium)

	code := readCode(file, itemOffset, medium)
	number := readInt(file, itemOffset, offsetItemNumber, medium) + 1
	revision := readInt(file, itemOffset, offsetItemRevision, medium)
	fullcode := fmt.Sprintf("G-%v-%04d-%03d", code, number, revision)

	strstr := format
	strstr = strings.Replace(strstr, "{code}", fullcode, 1)
	strstr = strings.Replace(strstr, "{brand}", brand, 1)
	strstr = strings.Replace(strstr, "{name}", name, 1)
	strstr = strings.Replace(strstr, "{author}", author, 1)

	if debug {
		fmt.Printf("%v\n", strstr)
	}

	fileName := fmt.Sprintf("%v/%v.mio", dirPath, strstr)

	// if fileName exists, return
	if _, err := os.Stat(fileName); err == nil {
		if !overwrite {
			if debug {
				fmt.Printf("File %v already exists; skipping\n", fileName)
			}
			return
		}
	}

	file2, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("error creating file: %v\n", err)
		return
	}
	data := make([]byte, medium.dataSize)
	_, _ = file.ReadAt(data, int64(getItemPos(itemOffset, medium)))
	file2.WriteAt(data, 0)
	file2.Close()

}

func getItemPos(itemOffset byte, medium *medium) uint32 {
	return medium.dataStart + uint32(itemOffset-1)*medium.dataSize
}

func readBrand(file *os.File, itemOffset byte, medium *medium) string {
	return readString(file, itemOffset, offsetBrand, lenBrand, medium)
}

func readName(file *os.File, itemOffset byte, medium *medium) string {
	return readString(file, itemOffset, offsetName, lenName, medium)
}

func readAuthor(file *os.File, itemOffset byte, medium *medium) string {
	return readString(file, itemOffset, offsetAuthor, lenAuthor, medium)
}

func readCode(file *os.File, itemOffset byte, medium *medium) string {
	return strings.ToUpper(readString(file, itemOffset, offsetCode, 4, medium))
}

func readString(file *os.File, itemOffset byte, dataOffset uint32, size int, medium *medium) string {
	datum := make([]byte, 0)
	offset := getItemPos(itemOffset, medium) + dataOffset
	for i := 0; i < size; i++ {
		b := make([]byte, 1)
		file.ReadAt(b, int64(offset+uint32(i)))
		if b[0] == 0 {
			if i == 0 {
				datum = []byte{'b', 'u', 'i', 'l', 't', 'i', 'n'}
			}
			break
		}
		datum = append(datum, b[0])
	}
	return string(datum)
}

func readInt(file *os.File, itemOffset byte, dataOffset uint32, medium *medium) int {
	bytes := make([]byte, 1)
	offset := getItemPos(itemOffset, medium) + dataOffset
	file.ReadAt(bytes, int64(offset))
	return int(bytes[0])
}

// make a function to compare two byte arrays
func compareBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
