package lua

import (
	"reflect"
	"testing"
)

func TestChannelMake(t *testing.T) {
	L := NewState()
	defer L.Close()
	errorIfScriptFail(t, L, `
    ch = channel.make()
    `)
	obj := L.GetGlobal("ch")
	ch, ok := obj.(LChannel)
	errorIfFalse(t, ok, "channel expected")
	errorIfNotEqual(t, 0, reflect.ValueOf(ch).Cap())
	close(ch)

	errorIfScriptFail(t, L, `
    ch = channel.make(10)
    `)
	obj = L.GetGlobal("ch")
	ch, _ = obj.(LChannel)
	errorIfNotEqual(t, 10, reflect.ValueOf(ch).Cap())
	close(ch)
}

func TestChannelSelect(t *testing.T) {
	L := NewState()
	defer L.Close()
	errorIfScriptFail(t, L, `ch = channel.make()`)
	errorIfScriptNotFail(t, L, `channel.select({1,2,3})`, "invalid select case")
	errorIfScriptNotFail(t, L, `channel.select({"<-|", 1, 3})`, "invalid select case")
	errorIfScriptNotFail(t, L, `channel.select({"<-|", ch, function() end})`, "can not send a function")
	errorIfScriptNotFail(t, L, `channel.select({"|<-", 1, 3})`, "invalid select case")
	errorIfScriptNotFail(t, L, `channel.select({"<-->", 1, 3})`, "invalid channel direction")
	errorIfScriptFail(t, L, `ch:close()`)
}
