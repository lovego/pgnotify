package handler

import (
	"reflect"
	"sync"

	"github.com/lovego/sorted_sets"
)

type Data struct {
	*sync.RWMutex
	// MapPtr is a pointer to a map to store data, required.
	MapPtr interface{}
	// MapKeys is the field names to get map keys from row struct, required.
	MapKeys []string
	// MapValue is the field name to get map value from row struct.
	// If it's empty, the row struct is use as map value.
	MapValue string

	// If the map value is a slice, it's used as sorted set. If it's a sorted set of struct,
	// SortedSetUniqueKey is required, it specifies the fields used as unique key.
	SortedSetUniqueKey []string

	// PrecondMethod is optional. It's a method name of row struct. It should be of "func () bool" form.
	// It is called before handling, if the return value is false, no handling is performed.
	PrecondMethod string

	// the map value to store data
	mapV reflect.Value
	// map value is a sorted set
	isSortedSets bool
	// real map value is a pointer of the row struct or row struct's {MapValue} field.
	realValueIsPointer bool
	// negative if no PrecondMethod present.
	precondMethodIndex int
}

func (d *Data) precond(row reflect.Value) bool {
	out := row.Method(d.precondMethodIndex).Call(nil)
	return out[0].Bool()
}

func (d *Data) save(row reflect.Value) {
	if d.precondMethodIndex >= 0 && !d.precond(row) {
		return
	}
	d.Lock()
	defer d.Unlock()

	mapV := d.mapV
	for i := 0; i < len(d.MapKeys)-1; i++ {
		key := row.FieldByName(d.MapKeys[i])
		value := mapV.MapIndex(key)
		if !value.IsValid() || value.IsNil() {
			value = reflect.MakeMap(mapV.Type().Elem())
			mapV.SetMapIndex(key, value)
		}
		mapV = value
	}

	key := row.FieldByName(d.MapKeys[len(d.MapKeys)-1])
	value := row
	if d.MapValue != "" {
		value = row.FieldByName(d.MapValue)
	}
	if d.realValueIsPointer {
		value = value.Addr()
	}
	if d.isSortedSets {
		value = sorted_sets.Save(mapV.MapIndex(key), value, d.SortedSetUniqueKey...)
	}
	mapV.SetMapIndex(key, value)
}

func (d *Data) remove(row reflect.Value) {
	if d.precondMethodIndex >= 0 && !d.precond(row) {
		return
	}
	d.Lock()
	defer d.Unlock()

	mapV := d.mapV
	for i := 0; i < len(d.MapKeys)-1; i++ {
		key := row.FieldByName(d.MapKeys[i])
		mapV := mapV.MapIndex(key)
		if !mapV.IsValid() || mapV.IsNil() {
			return
		}
	}
	key := row.FieldByName(d.MapKeys[len(d.MapKeys)-1])
	if d.isSortedSets {
		slice := mapV.MapIndex(key)
		if !slice.IsValid() {
			return
		}
		value := row
		if d.MapValue != "" {
			value = row.FieldByName(d.MapValue)
		}
		slice = sorted_sets.Remove(slice, value, d.SortedSetUniqueKey...)
		if !slice.IsValid() || slice.Len() == 0 {
			mapV.SetMapIndex(key, reflect.Value{})
		} else {
			mapV.SetMapIndex(key, value)
		}
	} else {
		mapV.SetMapIndex(key, reflect.Value{})
	}
}

func (d *Data) clear() {
	d.Lock()
	defer d.Unlock()
	d.mapV.Set(reflect.MakeMap(d.mapV.Type()))
}