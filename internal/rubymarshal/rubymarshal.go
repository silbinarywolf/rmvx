package rubymarshal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// This is the version RPG Maker VX Ace programs seemingly use for the latest
// Steam build (after inspecting in a hex editor)
const (
	supportedMajorVersion = 4
	supportedMinorVersion = 8
)

const (
	typeNull        = '0'
	typeTrue        = 'T'
	typeFalse       = 'F'
	typeArray       = '['
	typeObject      = 'o'
	typeSymbol      = ':'
	typeSymbolLink  = ';'
	typeFixNum      = 'i'
	typeIVar        = 'I'
	typeString      = '"'
	typeUserDefined = 'u'
	typeHash        = '{'
	typeFloat       = 'f'
	// note(jae): 2021-06-13
	// unimplemented
	// typeFixNum      = 'i'
	// typeBignum = 'l'
	// typeClass = 'c'
	// typeModule = 'm'
)

const (
	// note(jae): 2021-06-09
	// enable when developing only.
	// should not be on for releases
	devMode = true
)

type Decoder struct {
	r                  *bytes.Buffer
	symbols            []string
	userDefinedLoadMap map[string]func(data []byte, v reflect.Value)
	// topValue is stored so we can print it and debug the structure
	// while parsing
	topValue   interface{}
	savedError error
}

func NewDecoder(byteData []byte) *Decoder {
	s := &Decoder{}
	s.r = bytes.NewBuffer(byteData)
	s.userDefinedLoadMap = make(map[string]func(data []byte, v reflect.Value))
	return s
}

func (d *Decoder) Decode(v interface{}) (err error) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return errors.New("pointer required for Decode")
	}
	if !val.Elem().CanSet() {
		switch val.Type().Elem().Kind() {
		case reflect.Struct:
			return errors.New("pointer must either be not nil or you need to provide a pointer-to-pointer (&ptr)")
		default:
			return errors.New("unable to call Set on given type")
		}
	}
	if val.Type().Elem().Kind() == reflect.Ptr {
		// todo(Jae): 2021-06-13
		// remove this if we touch-up code to work with pointer-to-pointer
		// ie. add an indirect() function so we can support "*Struct" not just "Struct"
		return errors.New("pointer-to-pointer indirection not implemented")
	}
	major, err := d.r.ReadByte()
	if err != nil {
		return errors.New("cant decode MAJOR version")
	}
	minor, err := d.r.ReadByte()
	if err != nil {
		return errors.New("cant decode MINOR version")
	}
	if major != supportedMajorVersion || minor > supportedMinorVersion {
		return errors.New("unsupported marshal version")
	}
	d.topValue = v
	if err := d.parseRootTypeAndRecoverPanic(val); err != nil {
		return err
	}
	if d.savedError != nil {
		return d.savedError
	}
	// DEBUG: Print top value
	//log.Printf("%s", d.debugTopValue())
	d.topValue = nil
	return nil
}

// AddUserDefinedLoad will use the given callback to handle the class type data
//
// This was implemented so we could support RPG Maker VX Ace types such as "Table"
func (d *Decoder) AddUserDefinedLoad(className string, callback func(d []byte, v reflect.Value)) {
	if _, ok := d.userDefinedLoadMap[className]; ok {
		panic("cannot add same user defined type more than once: " + className)
	}
	d.userDefinedLoadMap[className] = callback
}

// debugTopValue pretty prints the top value with JSON
func (d *Decoder) debugTopValue() string {
	dat, err := json.MarshalIndent(d.topValue, "", "    ")
	if err != nil {
		panic(err)
	}
	return string(dat)
}

func (d *Decoder) parseRootTypeAndRecoverPanic(val reflect.Value) (err error) {
	// allow disabling of catching panic() for development purposes
	if !devMode {
		defer func() {
			if r := recover(); r != nil {
				if re, ok := r.(*rubyError); ok {
					err = re
				} else {
					panic(r)
				}
			}
		}()
	}
	d.parseType(val)
	return
}

type invalidFloat64 struct {
	Value string
	Err   error
}

func (err *invalidFloat64) Error() string {
	return "failed to parse float64 (double) from ruby data: " + err.Err.Error()
}

type unexpectedType struct {
	Got      string
	Expected string
}

type rubyError struct {
	message string
}

func (err *rubyError) Error() string {
	return err.message
}

func newRubyError(message string) error {
	return &rubyError{message: message}
}

func (err *unexpectedType) Error() string {
	return "expected type " + err.Expected + " but got " + err.Got
}

func (d *Decoder) parseType(val reflect.Value) {
	switch kind := d.MustReadByte(); kind {
	case typeNull: // 0
		val = val.Elem()
		if !val.CanSet() {
			// skip if cannot set
			return
		}
		switch val.Kind() {
		case reflect.Interface, reflect.Struct, reflect.String, reflect.Int, reflect.Int32, reflect.Int64:
			val.Set(reflect.Zero(val.Type()))
		default:
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: "pointer, struct, interface, string or integer",
			})
		}
	case typeTrue: // 'T'
		val = val.Elem()
		if !val.CanSet() {
			// skip if cannot set
			return
		}
		switch val.Kind() {
		case reflect.Interface, reflect.Bool:
			val.Set(reflect.ValueOf(true))
		default:
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: "bool",
			})
		}
	case typeFalse: // 'F'
		val = val.Elem()
		if !val.CanSet() {
			// skip if cannot set
			return
		}
		switch val.Kind() {
		case reflect.Interface, reflect.Bool:
			val.Set(reflect.ValueOf(false))
		default:
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: "bool",
			})
		}
	case typeFloat:
		str := d.parseString()
		val = val.Elem()
		if !val.CanSet() {
			// skip if cannot set
			return
		}
		var floatingNumber float64
		switch str {
		case "nan":
			floatingNumber = math.NaN()
		case "inf":
			floatingNumber = math.MaxFloat64
		case "-inf":
			floatingNumber = -math.MaxFloat64
		default:
			// note(jae): 2021-07-11
			// sample data: 0.56659999999999999\0<6  <-  where \0 is a null-byte
			//
			// Not sure what to do with the "\0<6" part, so I'm just gonna
			// ignore it. Seems related to the load_mantissa code.
			// https://github.com/ruby/ruby/blob/e330bbeeb1bd70180e5f6b835f2a39488e6c2d42/marshal.c
			strLen := len(str)
			for i, c := range str {
				if c == 0 {
					strLen = i
					break
				}
			}
			str = str[:strLen]
			var err error
			floatingNumber, err = strconv.ParseFloat(str, 64)
			if err != nil {
				d.saveError(&invalidFloat64{Value: str, Err: err})
				return
			}
		}
		switch val.Kind() {
		case reflect.Interface:
			val.Set(reflect.ValueOf(floatingNumber))
		case reflect.Float64:
			val.SetFloat(floatingNumber)
		default:
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: "float64",
			})
		}
	case typeFixNum: // i
		intValue := d.parseInt()
		val := val.Elem()
		if !val.CanSet() {
			// skip if cannot set
			return
		}
		switch val.Kind() {
		case reflect.Interface:
			val.Set(reflect.ValueOf(int(intValue)))
		case reflect.Int, reflect.Int32, reflect.Int64:
			val.SetInt(int64(intValue))
		default:
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: "int, int32 or int64",
			})
		}
	case typeSymbol: // :
		symbol := d.parseSymbol()
		val.Elem().Set(reflect.ValueOf(symbol))
	case typeSymbolLink: // ;
		symbol := d.parseIndexAndLookupSymbol()
		val.Elem().Set(reflect.ValueOf(symbol))
	case typeArray: // [
		size := d.parseInt()
		switch val.Kind() {
		case reflect.Ptr:
			switch val := val.Elem(); val.Kind() {
			case reflect.Interface:
				if size == 0 {
					// show [] instead of nil when printing to JSON
					arr := make([]interface{}, 0)
					val.Set(reflect.ValueOf(arr))
					return
				}
				arr := make([]interface{}, size)
				val.Set(reflect.ValueOf(arr))
				for i := 0; i < size; i++ {
					var arrayItem interface{}
					d.parseType(reflect.ValueOf(&arrayItem))
					arr[i] = arrayItem
				}
				return
			case reflect.Slice:
				if size == 0 {
					// show [] instead of nil when printing to JSON
					val.Set(reflect.MakeSlice(val.Type(), 0, 0))
					return
				}
				newv := reflect.MakeSlice(val.Type(), size, size)
				val.Set(newv)
				if val.Type().Elem().Kind() == reflect.Ptr {
					for i := 0; i < size; i++ {
						d.parseType(val.Index(i))
					}
				} else {
					for i := 0; i < size; i++ {
						d.parseType(val.Index(i).Addr())
					}
				}
				return
			}
		}
		// if unable to parse array, store error and skip array data in parsing
		d.saveError(&unexpectedType{
			Got:      val.Kind().String(),
			Expected: "ptr interface or ptr slice",
		})
		var emptyInterface interface{}
		refEmptyInterface := reflect.ValueOf(&emptyInterface)
		for i := 0; i < size; i++ {
			d.parseType(refEmptyInterface)
		}
	case typeString: // "
		str := d.parseString()
		val = val.Elem()
		if val.Kind() != reflect.Interface && val.Kind() != reflect.String {
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: reflect.String.String(),
			})
			return
		}
		val.Set(reflect.ValueOf(str))
	case typeIVar: // I
		// Load ivar from type
		var ivarData string
		switch ivarKind := d.MustReadByte(); ivarKind {
		case typeString:
			ivarData = d.parseString()
		default:
			panic(newRubyError("expected symbol type '\"' but got '" + string(ivarKind) + "'"))
		}
		if symbolCharLen := d.parseInt(); symbolCharLen == 1 {
			_ = d.parseSymbolOrSymbolLink() // can be symbol or symbol link
			if boolKind := d.MustReadByte(); boolKind != typeTrue {
				panic(newRubyError("expected type 'T' but got '" + string(boolKind) + "'"))
			}
		}
		val := val.Elem()
		if val.Kind() != reflect.Interface && val.Kind() != reflect.String {
			d.saveError(&unexpectedType{
				Got:      val.Kind().String(),
				Expected: reflect.String.String(),
			})
			return
		}
		val.Set(reflect.ValueOf(ivarData))
	case typeObject: // o
		// read class name of object
		// ie. "RPG::Tileset"
		_ = d.parseSymbolOrSymbolLink()
		fieldCount := d.parseInt()

		// DEBUG: Print class name to help with debugging
		// log.Printf("Class name: %s\n", className)

		switch val.Kind() {
		case reflect.Ptr:
			val = val.Elem()
			if !val.CanSet() {
				// If object value can't be set, skip over it
				var emptyInterface interface{}
				refEmptyInterface := reflect.ValueOf(&emptyInterface)
				for i := 0; i < fieldCount; i++ {
					_ = d.parseSymbolOrSymbolLink()
					d.parseType(refEmptyInterface)
				}
				return
			}

			/*if val.IsNil() && val.Kind() == reflect.Struct {
				panic(val.Type().Elem().String())
				val.Set(reflect.New(val.Type().Elem()))
			}*/
			/*if val.Kind() == reflect.Ptr {
				switch underlyingElem := val.Elem(); underlyingElem.Kind() {
				case reflect.Interface, reflect.Struct:
					val = underlyingElem
				case reflect.Invalid:
					panic("object: invalid pointer given. did you pass a pointer to a pointer accidentally?")
				default:
					panic(fmt.Sprintf("object: expected pointer to interface or struct but got pointer to type: %s", underlyingElem.Kind().String()))
				}
			}*/
			switch val.Kind() {
			case reflect.Interface:
				obj := make(map[string]interface{}, fieldCount)
				val.Set(reflect.ValueOf(obj))
				for i := 0; i < fieldCount; i++ {
					fieldName := d.parseSymbolOrSymbolLink()
					var objectFieldValue interface{}
					d.parseType(reflect.ValueOf(&objectFieldValue))
					obj[fieldName] = objectFieldValue
				}
			case reflect.Struct:
				refType := val.Type()
				structLookup := getStructFieldMapFromType(refType)

				var unknownFields []string
				for i := 0; i < fieldCount; i++ {
					fieldName := d.parseSymbolOrSymbolLink()
					if structField, ok := structLookup[fieldName]; ok {
						refValue := val.FieldByIndex(structField.Index)
						prevErr := d.savedError
						d.parseType(refValue.Addr())
						// note(jae): 2021-06-12
						// prevError check is necessary to avoid scrambling of error information
						if err, ok := d.savedError.(*unexpectedType); ok && prevErr != err {
							// add context to error
							d.savedError = errors.New(refType.Name() + " struct has field \"" + structField.Name + "\" with type " + structField.Type.String() + ", but expected " + err.Expected)
						}
					} else {
						// note(jae): 2021-06-12
						// considered adding a DisallowUnknownFields flag
						// but I'd prefer that to be the default behaviour so... not
						// gonna bother
						unknownFields = append(unknownFields, fieldName)

						// parse by unused
						var unusedField interface{}
						d.parseType(reflect.ValueOf(&unusedField))
					}
				}
				if len(unknownFields) > 0 {
					panic(newRubyError(fmt.Sprintf("ruby: unknown object fields %v for struct %s", unknownFields, refType.String())))
				}
			default:
				d.saveError(&unexpectedType{
					Got:      val.Type().String(),
					Expected: reflect.Struct.String(),
				})
				// note(jae): 2021-07-09
				// debugging with this line sucks and doesn't bring up what the actual bug is.
				// I *think* saveError is what we want but I can't remember.
				//panic(fmt.Sprintf("object: \"%s\" reflection not supported in object context", val.Type()))
				return
			}
		case reflect.Struct:
			panic("object: struct support not implemented")
		case reflect.Map:
			panic("object: map support not implemented")
		default:
			panic(fmt.Sprintf("object: unhandled type: %T", val))
		}
	case typeHash: // {
		size := d.parseInt()

		switch val.Kind() {
		case reflect.Ptr:
			val := val.Elem()
			if !val.CanSet() {
				// If value can't be set, skip over it
				var emptyInterface interface{}
				refEmptyInterface := reflect.ValueOf(&emptyInterface)
				for i := 0; i < int(size); i++ {
					d.parseType(refEmptyInterface)
					d.parseType(refEmptyInterface)
				}
				return
			}
			switch val.Kind() {
			case reflect.Interface:
				hash := make(map[string]interface{}, size)
				val.Set(reflect.ValueOf(hash))
				for i := 0; i < int(size); i++ {
					var key interface{}
					d.parseType(reflect.ValueOf(&key))

					var value interface{}
					d.parseType(reflect.ValueOf(&value))
					switch key := key.(type) {
					case int:
						// note(jae): 2021-06-10
						// handle map ID keys for "MapInfos.rvdata2"
						hash[strconv.Itoa(key)] = value
					case string:
						hash[key] = value
					default:
						panic(newRubyError(fmt.Sprintf("hash: expected string or int type but got %T,", key)))
					}
				}
			case reflect.Map:
				mapType := val.Type()
				if val.IsNil() {
					newVal := reflect.MakeMapWithSize(mapType, size)
					val.Set(newVal)
				}
				for i := 0; i < int(size); i++ {
					var keyInterface interface{}
					key := reflect.ValueOf(&keyInterface)
					d.parseType(key)

					subValue := reflect.New(mapType.Elem())
					d.parseType(subValue)

					keyUnderlying := key.Elem().Elem()
					valueUnderlying := subValue.Elem()
					if valueUnderlying.Kind() == reflect.Ptr {
						valueUnderlying = valueUnderlying.Elem()
					}
					if got := keyUnderlying.Kind(); got != mapType.Key().Kind() {
						d.saveError(&unexpectedType{
							Got:      got.String(),
							Expected: mapType.String(),
						})
						continue
					}
					val.SetMapIndex(keyUnderlying, valueUnderlying)
				}
			case reflect.Struct:
				refType := val.Type()
				structLookup := getStructFieldMapFromType(refType)

				var unknownFields []string
				for i := 0; i < size; i++ {
					// Get field name
					var fieldName string
					{
						var fieldNameData interface{}
						d.parseType(reflect.ValueOf(&fieldNameData))
						switch fieldNameData := fieldNameData.(type) {
						case string:
							fieldName = fieldNameData
						default:
							panic(newRubyError(fmt.Sprintf("hash: unable to map ruby hashmap to struct (%s) as key type is: %T", refType.String(), fieldNameData)))
						}
					}

					if structField, ok := structLookup[fieldName]; ok {
						refValue := val.FieldByIndex(structField.Index)
						prevErr := d.savedError
						d.parseType(refValue.Addr())
						// note(jae): 2021-06-12
						// prevError check is necessary to avoid scrambling of error information
						if err, ok := d.savedError.(*unexpectedType); ok && prevErr != err {
							// add context to error, this helps with debugging
							d.savedError = errors.New(refType.Name() + " struct has field \"" + structField.Name + "\" with type " + structField.Type.String() + ", but expected " + err.Expected)
						}
					} else {
						// note(jae): 2021-06-12
						// considered adding a DisallowUnknownFields flag
						// but I'd prefer that to be the default behaviour so... not
						// gonna bother
						unknownFields = append(unknownFields, fieldName)

						// parse by unused
						var unusedField interface{}
						d.parseType(reflect.ValueOf(&unusedField))
					}
				}
				if len(unknownFields) > 0 {
					panic(newRubyError(fmt.Sprintf("ruby: unknown object fields %v for struct %s", unknownFields, refType.String())))
				}
			case reflect.Slice:
				d.saveError(&unexpectedType{
					Got:      val.Kind().String(),
					Expected: "map[string]interface{} or map[string|int]*customStructHere",
				})
				return
			default:
				panic(fmt.Sprintf("hash: unhandled inner type: %s", val.Kind().String()))
			}
		case reflect.Map:
			panic("hash: todo handle explicit map type")
		default:
			panic(fmt.Sprintf("hash: unhandled type: %s", val.Kind().String()))
		}
	case typeUserDefined: // u
		// read class name of user defined data
		// ie. "Table"
		className := d.parseSymbolOrSymbolLink()
		byteCount := d.parseInt()
		userDefinedData := d.r.Next(byteCount)

		//_ = className
		//_ = userDefinedData

		funcCallback := d.userDefinedLoadMap[className]
		if funcCallback == nil {
			panic("Unhandled user defined type: " + className)
		}
		funcCallback(userDefinedData, val)
	default:
		panic(errors.New("unimplemented type: '" + string(kind) + "' (byte: " + strconv.Itoa(int(kind)) + ")"))
	}
}

func getStructFieldMapFromType(structType reflect.Type) map[string]reflect.StructField {
	// note(jae): 2021-06-13
	// if we need to speed this up later we can cache it like encoding/json
	structLookup := make(map[string]reflect.StructField)
	structFieldCount := structType.NumField()
	for i := 0; i < structFieldCount; i++ {
		structField := structType.Field(i)
		lookupName := structField.Tag.Get("ruby")
		if lookupName != "" {
			structLookup[lookupName] = structField
		}
	}
	return structLookup
}

func (d *Decoder) parseSymbolOrSymbolLink() string {
	switch symKind := d.MustReadByte(); symKind {
	case typeSymbol:
		return d.parseSymbol()
	case typeSymbolLink:
		return d.parseIndexAndLookupSymbol()
	default:
		panic(errors.New("expected symbol type ':' or ';' but got '" + string(symKind) + "'"))
	}
}

func (d *Decoder) parseIndexAndLookupSymbol() string {
	index := d.parseInt()
	symbol := d.symbols[index]
	return symbol
}

func (d *Decoder) parseSymbol() string {
	symbol := d.parseString()
	d.symbols = append(d.symbols, symbol)
	return symbol
}

func (d *Decoder) parseString() string {
	len := d.parseInt()
	if len == 0 {
		// note(jae): 2021-06-09
		// end of "Tilesets.rvdata2" had an empty string
		return ""
	}
	str := make([]byte, len)
	if _, err := d.r.Read(str); err != nil {
		panic(err)
	}
	return string(str)
}

func (d *Decoder) parseInt() int {
	var result int
	b, _ := d.r.ReadByte()
	c := int(int8(b))
	if c == 0 {
		return 0
	} else if 5 < c && c < 128 {
		return c - 5
	} else if -129 < c && c < -5 {
		return c + 5
	}
	cInt8 := int8(b)
	if cInt8 > 0 {
		result = 0
		for i := int8(0); i < cInt8; i++ {
			n, _ := d.r.ReadByte()
			result |= int(uint(n) << (8 * uint(i)))
		}
	} else {
		result = -1
		c = -c
		for i := 0; i < c; i++ {
			n, _ := d.r.ReadByte()
			result &= ^(0xff << uint(8*i))
			result |= int(n) << uint(8*i)
		}
	}
	return result
}

// saveError saves the first err it is called with,
// for reporting at the end of the unmarshal.
func (d *Decoder) saveError(err error) {
	if d.savedError == nil {
		d.savedError = err
	}
}

func (d *Decoder) MustReadByte() byte {
	v, err := d.r.ReadByte()
	if err != nil {
		panic(err)
	}
	return v
}
