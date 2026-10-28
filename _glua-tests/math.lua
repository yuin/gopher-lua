assert(math.fmod(13.5, 2) == 1.5)
assert(math.pow(7, 2) == 49)

local ok, msg = pcall(function()
  math.max()
end)
assert(not ok and string.find(msg, "wrong number of arguments"))

local ok, msg = pcall(function()
  math.min()
end)
assert(not ok and string.find(msg, "wrong number of arguments"))

-- math.huge is the positive infinity, as in reference Lua 5.1
assert(math.huge == 1/0)
assert(-math.huge == -1/0)
assert(math.huge > 1e308)
assert(tostring(math.huge) == "inf")
assert(tostring(-math.huge) == "-inf")

-- the fractional part returned by math.modf prints like the reference
local _, fr = math.modf(3.7)
assert(tostring(fr) == "0.7")
