
local ok, msg = pcall(function()
  string.dump()
end)
assert(not ok and string.find(msg, "GopherLua does not support the string.dump"))
assert(string.find("","aaa") == nil)
assert(string.gsub("hello world", "(%w+)", "%1 %1 %c") == "hello hello %c world world %c")

local ret1, ret2, ret3, ret4 = string.find("aaa bbb", "(%w+())")
assert(ret1 == 1)
assert(ret2 == 3)
assert(ret3 == "aaa")
assert(ret4 == 4)

-- string.format: Lua 5.1 semantics for %s, %c and %q

-- %s accepts only strings and numbers
assert(string.format("%s", 12) == "12")
assert(string.format("%s|%s", 12, 0.5) == "12|0.5")
assert(string.format("%s", 1e15) == "1e+15")
assert(not pcall(string.format, "%s", nil))
assert(not pcall(string.format, "%s", true))
assert(not pcall(string.format, "%s", {}))

-- %s stops at an embedded NUL byte ...
local withnul = "a" .. string.char(0) .. "b"
assert(#string.format("%s", withnul) == 1)
-- ... unless the string is 100 bytes or longer and no precision is given,
-- in which case it is passed through whole
local long = string.rep("x", 60) .. string.char(0) .. string.rep("y", 60)
assert(#string.format("%s", long) == 121)
assert(#string.format("%.9s", long) == 9)
local plain = string.rep("ab", 60)
assert(string.format("%99s", plain) == plain)
assert(#string.format("%99s", "x") == 99)
assert(string.format("%.4s", plain) == "abab")

-- %s width and precision count bytes, not characters
local twobyte = string.char(195, 169) -- a two-byte UTF-8 sequence
assert(#string.format("%.1s", twobyte) == 1)
assert(#string.format("%.2s", twobyte) == 2)
assert(string.format("%3s|", twobyte) == " " .. twobyte .. "|")
assert(#string.format("%4s", twobyte .. twobyte) == 4)

-- %c writes its value as a single byte; byte zero adds nothing
assert(string.format("%c", 72) == "H")
assert(string.format("a%cb", 0) == "ab")
assert(#string.format("%c", 256) == 0)
assert(string.format("%-3c|", 65) == "A  |")

-- %q escapes: '"', '\\' and newline keep their backslash (the newline stays
-- a real newline), '\r' becomes \r, NUL becomes the three-digit \000;
-- %q also accepts numbers and ignores any width
assert(string.format("%q", "a\nb") == '"a\\\nb"')
assert(string.format("%q", "a\rb") == '"a\\rb"')
assert(string.format("%q", "x" .. string.char(0) .. "y") == '"x\\000y"')
assert(string.format("%q", 42) == '"42"')
assert(string.format("%5q", "x") == '"x"')
assert(#string.format("%q", string.rep("x", 150)) == 152)
assert(not pcall(string.format, "%q", {}))

-- %q output always round-trips through loadstring
for i = 0, 255 do
  local orig = "a" .. string.char(i) .. "b"
  local f = loadstring("return " .. string.format("%q", orig))
  assert(f ~= nil and f() == orig)
end
