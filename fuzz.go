// +build gofuzz

package lua

func Fuzz(input []byte) int {
	vm := NewState()
	defer vm.Close()
	err := vm.DoString(string(input))
	if err != nil {
		return 0
	}
	return 1
}
