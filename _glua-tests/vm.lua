for i, v in ipairs({"hoge", {}, function() end, true, nil}) do
  local ok, msg = pcall(function()
    print(-v)
  end)
  assert(not ok and string.find(msg, "__unm undefined"))
end

assert(#"abc" == 3)
local tbl = {1,2,3}
setmetatable(tbl, {__len = function(self)
  return 10
end})
assert(#tbl == 10)

setmetatable(tbl, nil)
assert(#tbl == 3)

local ok, msg = pcall(function()
  return 1 < "hoge"
end)
assert(not ok and string.find(msg, "attempt to compare number with string"))

local ok, msg = pcall(function()
  return {} < (function() end)
end)
assert(not ok and string.find(msg, "attempt to compare table with function"))

local ok, msg = pcall(function()
  for n = nil,1 do
    print(1)
  end
end)
assert(not ok and string.find(msg, "for statement init must be a number"))

local ok, msg = pcall(function()
  for n = 1,nil do
    print(1)
  end
end)
assert(not ok and string.find(msg, "for statement limit must be a number"))

local ok, msg = pcall(function()
  for n = 1,10,nil do
    print(1)
  end
end)
assert(not ok and string.find(msg, "for statement step must be a number"))

local ok, msg = pcall(function()
  return {} + (function() end)
end)
assert(not ok and string.find(msg, "cannot perform add operation between table and function"))

local ok, msg = pcall(function()
  return {} .. (function() end)
end)
assert(not ok and string.find(msg, "cannot perform concat operation between table and function"))

-- test table with initial elements over 511
local bigtable = {1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,
                  1,1,1,1,1,1,1,1,1,1,1,1,10}
assert(bigtable[601] == 10)
