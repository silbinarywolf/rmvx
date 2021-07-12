package rmvx

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/silbinarywolf/rmvx/internal/rubymarshal"
)

const testDataDirectory = "testdata"

// flagUpdateTestData is set to true if the user runs "go test -u"
//
// This outputs the *_output files in the testdata automatically
var flagUpdateTestdata bool

func init() {
	flag.BoolVar(&flagUpdateTestdata, "u", false, "flag will update testdata *_output_* files when you run \"go test -u\"")
}

type osFS struct {
	dir string
}

func (filesystem *osFS) Open(name string) (fs.File, error) {
	var path string
	if name == "." {
		path = filesystem.dir
	} else {
		path = filesystem.dir + "/" + name
	}
	if !fs.ValidPath(path) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: path,
			Err:  errors.New("invalid path"),
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "open",
			Path: path,
			Err:  err,
		}
	}
	return f, nil
}

// note(jae: 2021-06-13
// this currently fails with our osFS impl. but takes a while to run
// not going to investigate yet.
//
// I fixed this in my own codebase but I don't really care about FS correctness
// for this library
//func TestOSFS(t *testing.T) {
//	if err := fstest.TestFS(&osFS{dir: testDataDirectory}, "Game.rvproj2"); err != nil {
//		t.Fatal(err)
//	}
//}

func TestLoadProjectWeakly(t *testing.T) {
	project, err := LoadProject(&osFS{dir: testDataDirectory})
	if err != nil {
		t.Fatalf("failed to load RPG Maker VX Ace project: %s", err)
	}
	if len(project.tilesets) == 0 {
		t.Fatal("expected at least 1 tileset in project data")
	}
	if len(project.mapInfos) == 0 {
		t.Fatal("expected at least 1 map in project data")
	}
	if len(project.Actors) == 0 {
		t.Fatal("expected at least 1 actor in project data")
	}
	if len(project.System.ArmorTypes) == 0 {
		t.Fatal("expected at least 1 armor type in project data")
	}
}

func TestLoadMap(t *testing.T) {
	inputFilename := "Map001.rvdata2"
	input, err := readEntireRMDataFile(inputFilename)
	if err != nil {
		t.Fatal(err)
	}

	// Interface
	var interfaceValue interface{}
	assertDecodeMatchesJSON(t, inputFilename, input, &interfaceValue)

	// Struct
	var value Map
	assertDecodeMatchesJSON(t, inputFilename, input, &value)

	// todo(jae): 2021-06-13
	// support for indirection with pointer types
	//{
	//	var v *Map
	//	assertDecodeMatchesJSON(t, inputFilename, input, v)
	//}
}

func TestLoadMapInfos(t *testing.T) {
	inputFilename := "MapInfos.rvdata2"
	input, err := readEntireRMDataFile(inputFilename)
	if err != nil {
		t.Fatal(err)
	}

	// Interface
	var interfaceValue interface{}
	assertDecodeMatchesJSON(t, inputFilename, input, &interfaceValue)

	// HashMap of MapInfo
	var value map[int]MapInfo
	assertDecodeMatchesJSON(t, inputFilename, input, &value)
}

func TestLoadTilesets(t *testing.T) {
	inputFilename := "Tilesets.rvdata2"
	input, err := readEntireRMDataFile(inputFilename)
	if err != nil {
		t.Fatal(err)
	}

	// Interface
	var interfaceValue interface{}
	assertDecodeMatchesJSON(t, inputFilename, input, &interfaceValue)

	// Slice of Tilesets
	var value []Tileset
	assertDecodeMatchesJSON(t, inputFilename, input, &value)
}

func TestLoadSystem(t *testing.T) {
	inputFilename := "System.rvdata2"
	input, err := readEntireRMDataFile(inputFilename)
	if err != nil {
		t.Fatal(err)
	}

	// Interface
	var interfaceValue interface{}
	assertDecodeMatchesJSON(t, inputFilename, input, &interfaceValue)

	// Struct
	var value System
	assertDecodeMatchesJSON(t, inputFilename, input, &value)
}

func TestLoadActors(t *testing.T) {
	inputFilename := "Actors.rvdata2"
	input, err := readEntireRMDataFile(inputFilename)
	if err != nil {
		t.Fatal(err)
	}

	// Interface
	var interfaceValue interface{}
	assertDecodeMatchesJSON(t, inputFilename, input, &interfaceValue)

	// Struct
	var value []Actor
	assertDecodeMatchesJSON(t, inputFilename, input, &value)
}

func readEntireRMDataFile(filename string) ([]byte, error) {
	f, err := os.Open("testdata/Data/" + filename)
	if err != nil {
		return nil, err
	}
	bytesData, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	return bytesData, nil
}

func assertDecodeMatchesJSON(t *testing.T, fileName string, input []byte, v interface{}) {
	ref := reflect.ValueOf(v)
	if ref.Kind() != reflect.Ptr {
		t.Fatalf("Invalid test parameter. Must pass pointer")
	}
	if ref.Type().Elem().Kind() == reflect.Ptr {
		t.Fatalf("Pointer-to-pointer is not currently supported in underlying Decode function")
	}
	// ie. interface, struct
	typeName := ref.Type().Elem().Kind().String()

	d := rubymarshal.NewDecoder(input)
	d.AddUserDefinedLoad("Table", loadTable)
	d.AddUserDefinedLoad("Tone", loadTone)
	if err := d.Decode(v); err != nil {
		t.Fatal(err)
	}
	output, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	baseNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	outputFilename := baseNameNoExt + "_output_" + typeName + ".json"
	if flagUpdateTestdata {
		if err := os.WriteFile("testdata/Data/"+outputFilename, output, 0666); err != nil {
			t.Fatal(err)
		}
	}
	expectedOutput, err := readEntireRMDataFile(outputFilename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected output to equal contents of \"%s\"", outputFilename)
	}
}
