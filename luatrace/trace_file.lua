-- Write luatrace traces to a file. Each line is one of
-- [S>] <filename> <linedefined> <lastlinedefined>  -- Start or call into a trace at filename somewhere in the function defined at linedefined
-- <                                            -- Return from a function
-- R <thread_id>                                -- Resume the thread thread_id
-- Y                                            -- Yield
-- P                                            -- pcall - the current line is protected for the duration of the following call
-- E                                            -- Error - unwind the stack until you find a p.
-- <linenumber> <microseconds>                  -- Accumulate microseconds against linenumber
-- Usually, a line will have time accumulated to it before and after it calls a function, so
-- function b() return 1 end
-- function c() return 2 end
-- a = b() + c()
-- will be traced as
-- 3 (time)
-- > (file) 1 1
-- 1 (time)
-- <
-- 3 (time)
-- > (file) 2 2
-- 2 (time)
-- <
-- 3 (time)


local DEFAULT_TRACE_LIMIT = 10000       -- How many traces to store before writing them out
local DEFAULT_TRACE_FILE_NAME = "trace-out.txt"
                                        -- What to call the trace file

-- Maybe these should be fields of a trace-file table.
local traces                            -- Array of traces
local count                             -- Number of traces
local limit                             -- How many traces we'll hold before we write them to...
local file                              -- The file to write traces to


-- Write traces to a file ------------------------------------------------------

local function write_trace(a, b, c, d)
  if type(a) == "number" then
    file:write(tonumber(a), " ", ("%g"):format(tonumber(b)), "\n")
  elseif a:match("[>ST]") then
    file:write(a, " ", tostring(b), " ", tostring(c), " ", tostring(d), "\n")
  elseif a == "R" then
    file:write("R ", tostring(b), "\n")
  else                                  -- It's one of <, Y, P or E
    file:write(a, "\n")
  end
end


local function write_traces()
  for i = 1, count do
    local t = traces[i]
    write_trace(t[1], t[2], t[3], t[4])
  end
  count = 0
end


-- API -------------------------------------------------------------------------

local trace_file = {}

local defaults =
{
  trace_file_name = DEFAULT_TRACE_FILE_NAME,
  trace_limit = DEFAULT_TRACE_LIMIT
}

local function get_settings(s)
  s = s or {}
  for k, v in pairs(defaults) do
    if not s[k] then s[k] = v end
  end
  return s
end


function trace_file.record(a, b, c, d)
  if limit < 2 then
    write_trace(a, b, c, d)
  else
    count = count + 1
    traces[count] = { a, b, c, d }
    if count > limit then write_traces() end
  end
end


function trace_file.open(settings)
  settings = get_settings(settings)

  if settings.trace_file then
    file = settings.trace_file
  else
    file = assert(io.open(settings.trace_file_name, "w"), "Couldn't open trace file")
  end

  limit = settings.trace_limit

  count, traces = 0, {}
end


function trace_file.close()
  if file then
    write_traces()
    file:close()
    file = nil
  end
end


function trace_file.read(settings)
  local do_not_close_file
  
  settings = get_settings(settings)

  if settings.trace_file then
    file = settings.trace_file
    do_not_close_file = true
  else
    file = assert(io.open(settings.trace_file_name, "r"), "Couldn't open trace file")
  end

  local recorder = settings.recorder

  recorder.open(settings)
  for l in file:lines() do
    local l1 = l:sub(1, 1)
    if l1:match("[>ST]") then
      local filename, linedefined, lastlinedefined = l:match(". (%S+) (%d+) (%d+)")
      recorder.record(l1, filename, tonumber(linedefined), tonumber(lastlinedefined))
    elseif l1 == "R" then
      local thread_id = l:match(". (%d+)")
      recorder.record(l1, tonumber(thread_id))
    elseif l1:match("[<YPE]") then
      recorder.record(l1)
    else
      local line, time = l:match("(%d+) (%d+%.*%d*)")
      if line then
        recorder.record(tonumber(line), tonumber(time))
      end
    end
  end
  recorder.close()
  if not do_not_close_file then
    file:close()
  end
end


return trace_file


-- EOF -------------------------------------------------------------------------

