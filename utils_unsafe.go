// +build !js,!appengine,!safe

package lua

import (
	"reflect"
	"unsafe"
)

func unsafeFastStringToReadOnlyBytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{sh.Data, sh.Len, sh.Len}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
