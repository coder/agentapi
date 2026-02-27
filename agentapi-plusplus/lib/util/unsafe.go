package util

import (
	"reflect"
	"unsafe"
)

// Based on https://stackoverflow.com/a/60598827
func GetUnexportedField[T any](obj *T, fieldName string) any {
	field := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}
