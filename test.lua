-- local debug = require("debug")
debug.sethook(function (event, line)
    print("event: " .. event .. " line: " .. line)
end, "l");
