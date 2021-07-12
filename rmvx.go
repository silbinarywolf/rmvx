// todo(jae): 2021-06-07
// think of a better name, add ideas in this comment and settle on one before release
package rmvx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math"
	"reflect"
	"strconv"

	"github.com/silbinarywolf/rmvx/internal/rubymarshal"
)

var ErrInvalidProject = errors.New("invalid project")

// Table is a user-defined Ruby type for RPG Maker VX Ace
type Table struct {
	X    int32
	Y    int32
	Z    int32
	Data []int16
}

func (table *Table) Get(x, y, z int) int16 {
	return table.Data[(table.X*table.Y*int32(z))+(table.X*int32(y))+int32(x)]
}

func (table *Table) GetInt32(x, y, z int32) int16 {
	return table.Data[(table.X*table.Y*z)+(table.X*y)+x]
}

func (table *Table) GetWrappedInt32(x, y, z int32) int16 {
	var xWrapped int32
	if x%table.X < 0 {
		xWrapped = x + table.X
	} else {
		xWrapped = x
	}
	var yWrapped int32
	if y%table.Y < 0 {
		yWrapped = y + table.Y
	} else {
		yWrapped = y
	}
	return table.GetInt32(xWrapped, yWrapped, z)
}

type Project struct {
	System System
	// Actors is a slice of actor data where the 0th entry is empty due to how RMVX stores data
	Actors []Actor

	fs       fs.FS
	tilesets []Tileset
	mapInfos map[int]MapInfo
}

type Tileset struct {
	ID           int      `ruby:"@id"`
	Name         string   `ruby:"@name"`
	Mode         int      `ruby:"@mode"`
	Note         string   `ruby:"@note"`
	TilesetNames []string `ruby:"@tileset_names"`
	Flags        Table    `ruby:"@flags"`
}

type MapInfo struct {
	Expanded bool   `ruby:"@expanded"`
	Name     string `ruby:"@name"`
	Order    int    `ruby:"@order"`
	ParentID int    `ruby:"@parent_id"`
	ScrollX  int    `ruby:"@scroll_x"`
	ScrollY  int    `ruby:"@scroll_y"`
}

type Map struct {
	TilesetID         int              `ruby:"@tileset_id"`
	ParallaxName      string           `ruby:"@parallax_name"`
	ParallaxShow      bool             `ruby:"@parallax_show"`
	ParallaxLoopX     bool             `ruby:"@parallax_loop_x"`
	ParallaxLoopY     bool             `ruby:"@parallax_loop_y"`
	ParallaxSX        int              `ruby:"@parallax_sx"`
	ParallaxSY        int              `ruby:"@parallax_sy"`
	ScrollType        int              `ruby:"@scroll_type"`
	SpecifyBattleback bool             `ruby:"@specify_battleback"`
	Width             int              `ruby:"@width"`
	Height            int              `ruby:"@height"`
	AutoplayBGM       bool             `ruby:"@autoplay_bgm"`
	AutoplayBGS       bool             `ruby:"@autoplay_bgs"`
	BGM               BackgroundSound  `ruby:"@bgm"`
	BGS               BackgroundSound  `ruby:"@bgs"`
	Battleback1_Name  string           `ruby:"@battleback1_name"`
	Battleback2_Name  string           `ruby:"@battleback2_name"`
	Note              string           `ruby:"@note"`
	DisplayName       string           `ruby:"@display_name"`
	DisableDashing    bool             `ruby:"@disable_dashing"`
	EncounterStep     int              `ruby:"@encounter_step"`
	Data              Table            `ruby:"@data"`
	Events            map[int]MapEvent `ruby:"@events"`
	EncounterList     []MapEncounter   `ruby:"@encounter_list"`
}

type MapEventGraphic struct {
	CharacterIndex int    `ruby:"@character_index"`
	CharacterName  string `ruby:"@character_name"`
	Direction      int    `ruby:"@direction"`
	Pattern        int    `ruby:"@pattern"`
	Tile           int    `ruby:"@tile_id"`
}

type EventCommand struct {
	Code       int           `ruby:"@code"`
	Indent     int           `ruby:"@indent"`
	Parameters []interface{} `ruby:"@parameters"`
}

type MoveRoute struct {
	List      []MoveRouteItem `ruby:"@list"`
	Repeat    bool            `ruby:"@repeat"`
	Skippable bool            `ruby:"@skippable"`
	Wait      bool            `ruby:"@wait"`
}

type MoveRouteItem struct {
	Code       int           `ruby:"@code"`
	Parameters []interface{} `ruby:"@parameters"`
}

type MapEventPage struct {
	DirectionFix  bool             `ruby:"@direction_fix"`
	MoveSpeed     int              `ruby:"@move_speed"`
	MoveType      int              `ruby:"@move_type"`
	PriorityType  int              `ruby:"@priority_type"`
	Through       bool             `ruby:"@through"`
	Trigger       int              `ruby:"@trigger"`
	StepAnime     bool             `ruby:"@step_anime"`
	WalkAnime     bool             `ruby:"@walk_anime"`
	MoveFrequency int              `ruby:"@move_frequency"`
	Condition     MapPageCondition `ruby:"@condition"`
	Graphic       MapEventGraphic  `ruby:"@graphic"`
	List          []EventCommand   `ruby:"@list"`
	MoveRoute     MoveRoute        `ruby:"@move_route"`
}

type MapPageCondition struct {
	ActorID    int  `ruby:"@actor_id"`
	ActorValid bool `ruby:"@actor_valid"`
	ItemID     int  `ruby:"@item_id"`
	ItemValid  bool `ruby:"@item_valid"`
	// SelfSwitchCH has a value of "A" by default and can be "A", "B", "C" or "D"
	SelfSwitchCH    string `ruby:"@self_switch_ch"`
	SelfSwitchValid bool   `ruby:"@self_switch_valid"`
	Switch1ID       int    `ruby:"@switch1_id"`
	Switch1Valid    bool   `ruby:"@switch1_valid"`
	Switch2ID       int    `ruby:"@switch2_id"`
	Switch2Valid    bool   `ruby:"@switch2_valid"`
	VariableID      int    `ruby:"@variable_id"`
	VariableValid   bool   `ruby:"@variable_valid"`
	VariableValue   int    `ruby:"@variable_value"`
}

type MapEncounter struct {
	TroopID   int         `ruby:"@troop_id"`
	RegionSet interface{} `ruby:"@region_set"`
	Weight    int         `ruby:"@weight"`
}

type MapEvent struct {
	ID    int            `ruby:"@id"`
	Name  string         `ruby:"@name"`
	X     int            `ruby:"@x"`
	Y     int            `ruby:"@y"`
	Pages []MapEventPage `ruby:"@pages"`
}

type BackgroundSound struct {
	Name   string `ruby:"@name" json:"name"`
	Pitch  int    `ruby:"@pitch" json:"pitch"`
	Volume int    `ruby:"@volume" json:"volume"`
}

type SystemVehicle struct {
	BGM            BackgroundSound `ruby:"@bgm" json:"bgm"`
	CharacterIndex int             `ruby:"@character_index" json:"characterIndex"`
	CharacterName  string          `ruby:"@character_name" json:"characterName"`
	StartMapID     int             `ruby:"@start_map_id" json:"startMapId"`
	StartX         int             `ruby:"@start_x" json:"startX"`
	StartY         int             `ruby:"@start_y" json:"startY"`
}

type SystemBattler struct {
	Level   int   `ruby:"@level" json:"level"`
	ActorID int   `ruby:"@actor_id" json:"actorId"`
	Equips  []int `ruby:"@equips" json:"equips"`
}

type System struct {
	// _
	// Not used in MV
	_ int `ruby:"@_"`
	// MagicNumber
	// Not used in MV
	MagicNumber int           `ruby:"@magic_number"`
	Boat        SystemVehicle `ruby:"@boat" json:"boat"`
	Ship        SystemVehicle `ruby:"@ship" json:"ship"`
	Airship     SystemVehicle `ruby:"@airship" json:"airship"`
	// ArmorTypes is a list of elements like "Physical", "Absorb", "Fire", etc
	// The first element is always "".
	ArmorTypes      []string        `ruby:"@armor_types" json:"armorTypes"`
	BattleBGM       BackgroundSound `ruby:"@battle_bgm" json:"battleBgm"`
	BattleEndMusic  BackgroundSound `ruby:"@battle_end_me" json:"victoryMe"`
	Battleback1Name string          `ruby:"@battleback1_name" json:"battleBack1Name"`
	Battleback2Name string          `ruby:"@battleback2_name" json:"battleBack2Name"`
	BattlerHue      int             `ruby:"@battler_hue" json:"battlerHue"`
	BattlerName     string          `ruby:"@battler_name" json:"battlerName"`
	CurrencyUnit    string          `ruby:"@currency_unit" json:"currencyUnit"`
	// EditMapID is the map the editor was last editing
	EditMapID int `ruby:"@edit_map_id" json:"editMapId"`
	// Elements is a list of elements like "Physical", "Absorb", "Fire", etc
	// The first element is always "".
	Elements      []string        `ruby:"@elements" json:"elements"`
	GameTitle     string          `ruby:"@game_title" json:"gameTitle"`
	GameoverMusic BackgroundSound `ruby:"@gameover_me"`
	Japanese      bool            `ruby:"@japanese"`
	// DisplayTP is true if displaying TP in battle
	DisplayTP   bool `ruby:"@opt_display_tp" json:"optDisplayTp"`
	DrawTitle   bool `ruby:"@opt_draw_title" json:"optDrawTitle"`
	ExtraExp    bool `ruby:"@opt_extra_exp" json:"optExtraExp"`
	FloorDeath  bool `ruby:"@opt_floor_death" json:"optFloorDeath"`
	Followers   bool `ruby:"@opt_followers" json:"optFollowers"`
	SlipDeath   bool `ruby:"@opt_slip_death" json:"optSlipDeath"`
	Transparent bool `ruby:"@opt_transparent" json:"optTransparent"`
	// UseMidi
	// Not used in MV
	UseMidi bool `ruby:"@opt_use_midi"`
	// PartyMembers is an array of character IDs
	PartyMembers []int             `ruby:"@party_members" json:"partyMembers"`
	SkillTypes   []string          `ruby:"@skill_types" json:"skillTypes"`
	Sounds       []BackgroundSound `ruby:"@sounds" json:"sounds"`
	StartMapID   int               `ruby:"@start_map_id" json:"startMapId"`
	StartX       int               `ruby:"@start_x" json:"startX"`
	StartY       int               `ruby:"@start_y" json:"startY"`
	// Switches is a list of the names given for switches
	// The first element is always "null" (technically empty string since we use []string)
	Switches []string `ruby:"@switches" json:"switches"`
	// Variables is list of the names given for variables
	// The first element is always "null" (technically empty string since we use []string)
	Variables []string `ruby:"@variables" json:"variables"`
	Terms     struct {
		// Basic is a list of stats:
		// "Level", "LV", "HP", "HP", "MP", "MP", "TP", "TP"
		Basic []string `ruby:"@basic" json:"basic"`
		// Commands is a list of battle commands:
		// "Fight", "Escape", "Attack", "Guard", "Items", "Skills" and more
		Commands []string `ruby:"@commands" json:"commands"`
		// ETypes is a list of item types:
		// "Weapon", "Shield", "Headgear", "Bodygear" and "Accessory"
		ETypes []string `ruby:"@etypes" json:"etypes"`
		// Params is a list of stats:
		// "MaxHP", "MaxMP", "ATK", "DEF", "MAT", "MDF", "AGI" and "LUK"
		Params []string `ruby:"@params" json:"params"`
	} `ruby:"@terms" json:"terms"`
	TestBattlers []SystemBattler `ruby:"@test_battlers" json:"testBattlers"`
	TestTroopID  int             `ruby:"@test_troop_id" json:"testTroopId"`
	Title1Name   string          `ruby:"@title1_name" json:"title1Name"`
	Title2Name   string          `ruby:"@title2_name" json:"title2Name"`
	TitleBGM     BackgroundSound `ruby:"@title_bgm" json:"titleBgm"`
	VersionID    int             `ruby:"@version_id" json:"versionId"`
	// WeaponTypes is a list of elements like "Axe", "Claw", "Spear", etc
	// The first element is always "".
	WeaponTypes []string `ruby:"@weapon_types" json:"weaponTypes"`
	WindowTone  Tone     `ruby:"@window_tone" json:"windowToneVX"`
}

type Actor struct {
	CharacterIndex int            `ruby:"@character_index" json:"characterIndex"`
	CharacterName  string         `ruby:"@character_name" json:"characterName"`
	ClassID        int            `ruby:"@class_id" json:"classId"`
	Description    string         `ruby:"@description" json:"description"`
	Equips         []int          `ruby:"@equips" json:"equips"`
	FaceIndex      int            `ruby:"@face_index" json:"faceIndex"`
	FaceName       string         `ruby:"@face_name" json:"faceName"`
	Features       []ActorFeature `ruby:"@features" json:"features"`
	ID             int            `ruby:"@id" json:"id"`
	InitialLevel   int            `ruby:"@initial_level" json:"initialLevel"`
	MaxLevel       int            `ruby:"@max_level" json:"maxLevel"`
	Name           string         `ruby:"@name" json:"name"`
	Nickname       string         `ruby:"@nickname" json:"nickname"`
	Note           string         `ruby:"@note" json:"note"`
}

type ActorFeature struct {
	Code   int     `ruby:"@code" json:"code"`
	DataID int     `ruby:"@data_id" json:"dataId"`
	Value  float64 `ruby:"@value" json:"value"`
}

func (project *Project) getMapFilenameByID(mapID int) (string, error) {
	if mapID == 0 {
		return "", errors.New("invalid map id: 0")
	}
	_, ok := project.mapInfos[mapID]
	if !ok {
		return "", fmt.Errorf("map does not exist: %d", mapID)
	}
	idPart := strconv.Itoa(mapID)
	switch len(idPart) {
	case 2:
		idPart = "0" + idPart
	case 1:
		idPart = "00" + idPart
	case 0:
		idPart = "000"
	}
	mapFilename := "Map" + idPart
	return mapFilename, nil
}

func (project *Project) GetTileset(id int) (*Tileset, error) {
	if id == 0 || id >= len(project.tilesets) {
		return nil, errors.New("invalid tileset id given, out of bounds")
	}
	tileset := &project.tilesets[id]
	return tileset, nil
}

func (project *Project) LoadMapByID(mapID int) (*Map, error) {
	mapFilename, err := project.getMapFilenameByID(mapID)
	if err != nil {
		return nil, err
	}
	var mapData Map
	if err := loadRMVXDataFile(project, mapFilename, &mapData); err != nil {
		return nil, err
	}
	return &mapData, nil
}

func loadRMVXDataFile(project *Project, assetName string, value interface{}) error {
	f, err := project.fs.Open("Data/" + assetName + ".rvdata2")
	if err != nil {
		return err
	}
	bytesData, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return err
	}
	d := rubymarshal.NewDecoder(bytesData)
	d.AddUserDefinedLoad("Table", loadTable)
	d.AddUserDefinedLoad("Tone", loadTone)
	if err := d.Decode(value); err != nil {
		return err
	}
	return nil
}

func LoadProject(fs fs.FS) (*Project, error) {
	// Load entrypoint file
	{
		f, err := fs.Open("Game.rvproj2")
		if err != nil {
			return nil, err
		}
		bytesData, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		if !bytes.HasPrefix(bytesData, []byte("RPGVXAce 1")) {
			return nil, ErrInvalidProject
		}
	}

	project := &Project{}
	project.fs = fs

	// Load tilesets
	if err := loadRMVXDataFile(project, "Tilesets", &project.tilesets); err != nil {
		return nil, err
	}

	// Load map infos
	if err := loadRMVXDataFile(project, "MapInfos", &project.mapInfos); err != nil {
		return nil, err
	}

	// Load System
	if err := loadRMVXDataFile(project, "System", &project.System); err != nil {
		return nil, err
	}

	// Load Actors
	if err := loadRMVXDataFile(project, "Actors", &project.Actors); err != nil {
		return nil, err
	}

	// DEBUG: Get generic output of data, this is used to figure out struct... structure.
	/* var v interface{}
	if err := loadRMVXDataFile(project, "CommonEvents", &v); err != nil {
		panic(err)
	}
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		panic(err)
	}
	panic(fmt.Sprintf("%s", b)) */

	return project, nil
}

type Tone struct {
	Red, Green, Blue int16
	Gray             int16
}

func loadTone(data []byte, val reflect.Value) {
	r := bytes.NewBuffer(data)

	// note(jae): 2021-06-18
	// RPG Maker VX Ace stores "Tone" as doubles even though
	// the values each only range between -255 and 255.
	//
	// I overhead conversations in "The Maple Shrine" (MKXP Discord)
	// that people generally don't adjust tone by a decimal point in Ruby scripts
	// and in the GUI, it's impossible to do so.
	//
	// This is why we just store them as int16s internally.
	red := mustReadFloat64(r)
	green := mustReadFloat64(r)
	blue := mustReadFloat64(r)
	gray := mustReadFloat64(r)
	value := Tone{
		Red:   int16(red),
		Green: int16(green),
		Blue:  int16(blue),
		Gray:  int16(gray),
	}
	switch val.Elem().Kind() {
	case reflect.Ptr, reflect.Interface:
		val.Elem().Set(reflect.ValueOf(&value))
	case reflect.Struct:
		val.Elem().Set(reflect.ValueOf(value))
	default:
		panic("loadTone: unhandled type: " + val.Elem().Kind().String())
	}
}

func loadTable(data []byte, val reflect.Value) {
	r := bytes.NewBuffer(data)

	// Read header
	var x, y, z int32
	var arrSize int
	{
		_ = mustReadInt32(r) // argument count
		x = mustReadInt32(r)
		y = mustReadInt32(r)
		z = mustReadInt32(r)
		sizeInt32 := mustReadInt32(r)
		if sizeInt32 != x*y*z {
			panic(errors.New("Table: bad file format"))
		}
		arrSize = int(sizeInt32)
	}

	// Read array
	tableData := make([]int16, arrSize)
	{
		// note(Jae): 2021-06-09
		// If we want to speed-up load times for LittleEndian machines,
		// we could just detect machine endianness and then do a copy()
		// (Original Ruby source code does a memcpy())
		for i := range tableData {
			tableData[i] = mustReadInt16(r)
		}
	}

	value := Table{
		X:    x,
		Y:    y,
		Z:    z,
		Data: tableData,
	}
	switch val.Elem().Kind() {
	case reflect.Ptr, reflect.Interface:
		val.Elem().Set(reflect.ValueOf(&value))
	case reflect.Struct:
		val.Elem().Set(reflect.ValueOf(value))
	default:
		panic("LoadTable: unhandled type: " + val.Elem().Kind().String())
	}
}

// mustReadFloat64 is a fast-path binary.LittleEndian.Read
func mustReadFloat64(r *bytes.Buffer) float64 {
	b := make([]byte, 8)
	if _, err := r.Read(b); err != nil {
		panic(err)
	}
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	uint64Value := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	return math.Float64frombits(uint64Value)
}

// mustReadInt32 is a fast-path binary.LittleEndian.Read
func mustReadInt32(r *bytes.Buffer) int32 {
	bs := make([]byte, 4)
	if _, err := r.Read(bs); err != nil {
		panic(err)
	}
	_ = bs[3] // bounds check hint to compiler; see golang.org/issue/14808
	return int32(uint32(bs[0]) | uint32(bs[1])<<8 | uint32(bs[2])<<16 | uint32(bs[3])<<24)
}

// mustReadInt16 is a fast-path binary.LittleEndian.Read
func mustReadInt16(r *bytes.Buffer) int16 {
	bs := make([]byte, 2)
	if _, err := r.Read(bs); err != nil {
		panic(err)
	}
	_ = bs[1] // bounds check hint to compiler; see golang.org/issue/14808
	return int16(uint16(bs[0]) | uint16(bs[1])<<8)
}
