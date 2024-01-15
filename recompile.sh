rm glua
# make glua
# ./glua luatrace.lua
go build cmd/glua/glua.go
# ./glua test/wrap-error.lua > test.txt 2>&1
# (base) ➜  luatrace git:(master) ✗ pwd
# /home/SENSETIME/wufei2/go/src/github.com/geoffleyland/luatrace

./glua test/tailcall.lua > test.txt 2>&1
luatrace.profile > glua_annotated-source.txt 2>&1
lua test/tailcall.lua > testlua.txt 2>&1
luatrace.profile > lua_annotated-source.txt 2>&1
# ./glua lua-callgrind.lua test/accumulate.lua
# kcachegrind lua-callgrind.txt