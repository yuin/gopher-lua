#/bin/bash
OLDPWD=`pwd`
myexit() {
  cd "${OLDPWD}"
  rm -f glua.exe
  rm -f glua
  exit $1
}
echo go build cmd/glua/glua.go
go build cmd/glua/glua.go
[ $? -ne 0 ] && {
   echo "compile failed."
   myexit 1
}

TESTS=("bugs.lua")
cd _glua-tests
for TEST in "${TESTS[@]}"; do
  echo "testing ${TEST}"
  ../glua -mx 20 ${TEST}
  [ $? -ne 0 ] && {
     echo "failed."
     myexit 1
  }
done
cd ../

TESTS=("attrib.lua" "calls.lua" "closure.lua" "constructs.lua" "events.lua"
       "literals.lua" "locals.lua" "math.lua" "sort.lua" "strings.lua" "vararg.lua"
       "pm.lua" "files.lua")
cd _lua5.1-tests
for TEST in "${TESTS[@]}"; do
  echo "testing ${TEST}"
  ../glua -mx 20 ${TEST}
  [ $? -ne 0 ] && {
     echo "failed."
     myexit 1
  }
done
cd ../

myexit 0
