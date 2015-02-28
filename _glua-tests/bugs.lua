
-- bug #10
local function inspect(options)
    options = options or {}
    return type(options)
end
assert(inspect(nil) == "table")
