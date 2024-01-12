rm glua
# make glua
# ./glua luatrace.lua
go build cmd/glua/glua.go
# ./glua test/wrap-error.lua > test.txt 2>&1                  
# (base) ➜  luatrace git:(master) ✗ pwd
# /home/SENSETIME/wufei2/go/src/github.com/geoffleyland/luatrace

LD_LIBRARY_PATH="/usr/local/lib/lua/5.1/luatrace" ./glua test/wrap-error.lua > test.txt 2>&1