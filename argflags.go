package argflags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ArgFlags uses its elements as command line arguments containing none or more flags, arguments starting with a dash.
// These 'named' flags usually have a following argument for a value.
// ArgFlags applies these flags values to ColumnNames in a given struct, converting the string argument to the respective type.
// It is used to assign values from the command line, directly to a structures fields.
// ArgFlags will detect the field type and convert the string argument value into that type.
// Any field supporting the encoding.TextUnmarshaler interface will have that interface used with the argument value as its text.
// ColumnNames may be 'tagged' with a 'flag' tag, the value of which is a comma delimited list of flag names to match to.
// e.g. MyNames []string `flag:"names,n"`    This will match to either the '-names' or '-n' flag value.
// Slices should be given in the commandline as a quoted, comma delimited list
// Sub Arguments
// Subargs are ColumnNames which contain their own Flag fields.
// When a struct wishes to expose one or more of its fields as flag structs, it uses the sugarg tag:
// e.g. OtherData *MyStruct `flag:"+"`  Flags will also match with any flag fields in 'OtherData' assuming MyStruct has public fields.
// Sub arg fields MUST be either a struct or a pointer to a struct.  nil pointers are instanciated when a matching flag is found.
type ArgFlags []string

// String returns the existing arguments as a space delimited list
func (args ArgFlags) String() string {
	return strings.Join(args, " ")
}

// FlagNames returns all the flag names, (arguments beginning with '-') found in the arguments.
func (args ArgFlags) FlagNames() []string {
	var names []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		names = append(names, strings.TrimLeft(arg, "-"))
	}
	return names
}

// ApplyTo applies the argument flags to the given struct pointer.
// str must be a pointer to a struct.
// Field names in the struct are matched to the named flags either directly to the field name or
// with a tag of 'flags:"one,two,three"'.  Any tag name can match to a flag.
// Fields should be base types, string, ints, floats, bools etc or slices of those.
// If a field contains an object supporting the TextUnmarshaler the argument value is passed to that interface.
// in the given arguments, named flags should always have a following argument for the value of the flag, except bool flags.
// Bool flags are defined by the Field in the strurct and can have optional values.
// Bool flags default to true
// If a bool flag has a value following it, it is tested to be a bool value (true or false), if not those, its ignored
func (args ArgFlags) ApplyTo(str interface{}) ([]string, error) {
	v, err := getStructValue(str)
	if err != nil {
		return nil, err
	}
	var unused []string
	var i int
	for ; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			unused = append(unused, arg)
			continue
		}
		fld, err := newFlagField(strings.TrimLeft(arg, "-"), *v)
		if err != nil {
			// no matching field for the flag, ignore it
			unused = append(unused, arg)
			continue
		}
		var argValue string
		vals := args[i+1:]
		if v, remain, err := findFlagValue(vals, fld.Type()); err != nil {
			return nil, fmt.Errorf("%s  %v", arg, err)
		} else {
			argValue = v
			// move along args, past any value found (can be zero movement)
			i += len(vals) - len(remain)
		}
		if err := fld.SetValue(argValue); err != nil {
			return nil, fmt.Errorf("'%s'  %v", arg, err)
		}
	}
	return unused, nil
}

func findFlagValue(args []string, fldType reflect.Type) (value string, remain []string, err error) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		value = args[0]
	}
	// bool flags have optional value.  only used if parsable as bool, otherwise defaults to true and ignores next arg
	if fldType.Kind() == reflect.Bool {
		// test if its parsable as bool
		_, err := strconv.ParseBool(value)
		if value == "" || err != nil {
			// argval not a bool, ignore it and return true
			return strconv.FormatBool(true), args, nil
		}
		// otherwise treat as regular value
	}
	if value == "" {
		return "", nil, fmt.Errorf("no value found")
	}
	return value, args[1:], nil
}

func getStructValue(str interface{}) (*reflect.Value, error) {
	if !isStructPointer(reflect.TypeOf(str)) {
		return nil, fmt.Errorf("flags can only be applied to a struct pointer")
	}
	v := reflect.ValueOf(str).Elem()
	if v.Kind() == reflect.Interface {
		v = v.Elem().Elem()
	}
	return &v, nil
}

func isStructPointer(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}
