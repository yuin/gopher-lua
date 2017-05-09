// +build js appengine safe

package lua

func unsafeFastStringToReadOnlyBytes(s string) []byte {
	return []byte(s)
}
