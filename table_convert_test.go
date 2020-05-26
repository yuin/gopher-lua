package lua

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

var testingConversionJSONValue interface{}
var testingConversionLuaValue LValue

func init() {
	resp, err := http.Get("https://raw.githubusercontent.com/valyala/fastjson/master/testdata/large.json")
	if err != nil {
		panic(err)
	}
	jsonBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(jsonBody, &testingConversionJSONValue)
	if err != nil {
		panic(err)
	}
	testingConversionLuaValue = ConvertToLValue(testingConversionJSONValue)
}

func BenchmarkToLValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ConvertToLValue(testingConversionJSONValue)
	}
}

func BenchmarkToGValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ConvertFromLValue(testingConversionLuaValue)
	}
}
