package argflags

import (
	"encoding"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
)

const FlagTagName = "flag"
const sliceDelimiter = ","

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// FlagField represents a Field in a struct which has been matched to a flag
type FlagField interface {
	Type() reflect.Type
	SetValue(value string) error
}

type flagField struct {
	fldValue reflect.Value
}

func (ff flagField) Type() reflect.Type {
	return ff.fldValue.Type()
}

func (ff flagField) SetValue(value string) error {
	return setValue(value, ff.fldValue)
}

func setValue(value string, fld reflect.Value) error {
	if tm := asTextUnmarshaler(fld); tm != nil {
		return tm.UnmarshalText([]byte(value))
	}
	t := fld.Type()
	switch fld.Type().Kind() {
	case reflect.Ptr:
		if fld.IsZero() || fld.IsNil() {
			fld.Set(reflect.New(t.Elem()))
		}
		return setValue(value, fld.Elem())
	case reflect.Slice:
		return setFieldSlice(strings.Split(value, sliceDelimiter), fld)
	}

	sv, err := stringToType(value, t)
	if err != nil {
		return err
	}
	fld.Set(reflect.ValueOf(sv))
	return nil
}

func stringToType(s string, t reflect.Type) (interface{}, error) {
	switch t.Kind() {
	case reflect.String:
		return s, nil
	case reflect.Bool:
		return strconv.ParseBool(s)
	case reflect.Int:
		return strconv.Atoi(s)
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return strconv.ParseInt(s, 64, 64)
	case reflect.Float64:
		return strconv.ParseFloat(s, 64)
	case reflect.Float32:
		return strconv.ParseFloat(s, 32)
	default:
		return nil, fmt.Errorf("%s is an unsupported field type", t.Name())
	}
}

// asTextUnmarshaler will return an instance of a textUnmarshaler if the given value supports that interface.
// If given value is not a pointer, a reference to the given address will be returned as the interface.
func asTextUnmarshaler(fld reflect.Value) encoding.TextUnmarshaler {
	fldPtr := fld
	if fld.Type().Kind() != reflect.Ptr {
		fldPtr = fld.Addr()
	}
	if !fldPtr.Type().Implements(textUnmarshalerType) {
		return nil
	}
	return fldPtr.Interface().(encoding.TextUnmarshaler)
}

func setFieldSlice(ss []string, fld reflect.Value) error {
	t := fld.Type()
	// TODO Check if value exist and append values
	inst := reflect.MakeSlice(t, len(ss), len(ss))
	for i, s := range ss {
		if err := setValue(s, inst.Index(i)); err != nil {
			return err
		}
	}
	fld.Set(inst)
	return nil
}

// findFieldIndex searches the given type for a matching flag field.
// The given type must be a sturct or pointer to one.
// If the given type contains a matching field, the index of that field is returned.
// If the given type has no matching field, but has subargs, these are searched.
// returns the indexes of each field, with the last index being the actual field.
func findFieldIndex(name string, t reflect.Type, parents []int) []int {
	if t.Kind() == reflect.Ptr {
		return findFieldIndex(name, t.Elem(), parents)
	}
	var subArgIndexes []int
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if strings.EqualFold(name, f.Name) {
			return append(parents, i)
		}
		tags := strings.Split(f.Tag.Get(FlagTagName), ",")
		if isNameInTag(name, tags) {
			return append(parents, i)
		}
		if isSubArgTag(tags) {
			fld := t.Field(i)
			if !isStructPointer(fld.Type) && fld.Type.Kind() != reflect.Struct {
				log.Panicf("Field %s in %s is tagged as a sub argument field '+', but is not a struct or pointer to a struct", fld.Name, t.String())
			}
			subArgIndexes = append(subArgIndexes, i)
		}
	}
	// Not found in given value, search fields marked as subargs (tag:+)
	for _, i := range subArgIndexes {
		p := append(parents, i)
		if is := findFieldIndex(name, t.Field(i).Type, p); len(is) > 0 {
			return is
		}
	}
	return nil
}

func isSubArgTag(tags []string) bool {
	for _, tag := range tags {
		if tag == "+" {
			return true
		}
	}
	return false
}

func isNameInTag(name string, tags []string) bool {
	for _, t := range tags {
		if t == "omitempty" || t == "-" || t == "+" {
			continue
		}
		if strings.EqualFold(t, name) {
			return true
		}
	}
	return false
}

func ensureNotNil(v reflect.Value, index []int) {
	if len(index) == 0 {
		return
	}
	fld := v.Field(index[0])
	t := fld.Type()
	if t.Kind() == reflect.Ptr {
		if fld.IsNil() {
			fld.Set(reflect.New(t.Elem()))
		}
		fld = fld.Elem()
	}
	ensureNotNil(fld, index[1:])
}

func findField(name string, v reflect.Value) (reflect.Value, error) {
	t := v.Type()
	index := findFieldIndex(name, t, nil)
	if len(index) == 0 {
		return reflect.Zero(reflect.TypeOf("")), fmt.Errorf("field %s not found in %s", name, t.String())
	}
	ensureNotNil(v, index)
	return v.FieldByIndex(index), nil
}

func newFlagField(name string, v reflect.Value) (FlagField, error) {
	fld, err := findField(name, v)
	if err != nil {
		return nil, err
	}
	return &flagField{fldValue: fld}, nil
}
