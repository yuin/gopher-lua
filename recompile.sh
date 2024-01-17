rm glua
# make glua
# ./glua luatrace.lua
go build cmd/glua/glua.go
# ./glua test/wrap-error.lua > test.txt 2>&1
# (base) ➜  luatrace git:(master) ✗ pwd
# /home/SENSETIME/wufei2/go/src/github.com/geoffleyland/luatrace
# test/tailcall.lua
# ./glua test/accumulate.lua > test.txt 2>&1
# luatrace.profile > glua_annotated-source.txt 2>&1
# luatrace.profile


lua test/accumulate.lua > testlua.txt 2>&1
luatrace.profile > lua_annotated-source.txt 2>&1




# ./glua lua-callgrind.lua test/accumulate.lua
# kcachegrind lua-callgrind.txt

# ./glua test/example.lua > example.txt 2>&1
# luatrace.profile > result/glua_example_annotated-source.txt 2>&1
# lua test/example.lua > testlua.txt 2>&1
# luatrace.profile > result/lua_example_annotated-source.txt 2>&1