local trace_file = require("luatrace.trace_file")

local source_files                              -- Map of source files we've seen
local functions                                 -- Map of functions
local lines                                     -- Map of lines

local threads                                   -- All the running threads
local thread_stack                              -- and the order they're running in

local stack                                     -- Call stack

local total_time                                -- Running total of all the time we've recorded

local trace_count                               -- How many traces we've seen (for reporting errors)
local error_count                               -- How many errors we've seen

local profile = {}


--------------------------------------------------------------------------------

function profile.open()
  source_files, functions, lines = {}, {}, {}
  local main_thread = { top=0 }
  threads = { main_thread }
  thread_stack = { main_thread, top=1 }
  stack = { top=0 }
  total_time = 0
  trace_count, error_count = 0, 0
end


-- Recording -------------------------------------------------------------------

local function get_source_file(filename)
  local source_file = source_files[filename]
  if not source_file then
    source_file = { filename=filename, lines = {}, functions={} }
    source_files[filename] = source_file
  end
  return source_file
end


local function get_function(filename, line_defined, last_line_defined)
  local name = filename..":"..tostring(line_defined).."-"..tostring(last_line_defined)
  local f = functions[name]
  if not f then
    local sf = get_source_file(filename)
    f = {
      name=name,
      source_file=sf,
      line_defined=line_defined,
      last_line_defined=last_line_defined,
      hits=0,
      self_time=0,
      child_time=0,
    }
    functions[name] = f
    sff = sf.functions[line_defined]
    if not sff then
      sf.functions[line_defined] = { f }
    else
      sff[#sff+1] = f
    end
  end
  return f
end


local function get_thread(thread_id)
  local thread = threads[thread_id]
  if not thread then
    thread = { top=0 }
    threads[thread_id] = thread
  end
  return thread
end


local function get_top()
  return stack[stack.top]
end


local function thread_top()
  return thread_stack[thread_stack.top]
end


local function get_line(line_number, frame)
  frame = frame or get_top()
  local file = frame.func.source_file
  local line = file.lines[line_number]
  if not line then
    line = { file=file, line_number=line_number, hits=0, self_time=0, child_time=0 }
    file.lines[line_number] = line
    lines[#lines+1] = line
  end
  return line
end


local function call(frame)
  local top = get_top()
  if top then
    top.line_start_time = total_time
    if top.current_line then
      local line = top.current_line
      top.line_self_time_at_start = line.self_time
      top.line_child_time_at_start = line.child_time
    end
  end
  stack.top = stack.top + 1
  stack[stack.top] = frame
  frame.func.hits = frame.func.hits + 1
  frame.func_start_time = total_time
  frame.func_self_time_at_start = frame.func.self_time
  frame.func_child_time_at_start = frame.func.child_time
end


local function call_on_thread(frame)
  local thread = thread_top()
  thread.top = thread.top + 1
  thread[thread.top] = frame
end


local function push_thread(thread)
  thread_stack.top = thread_stack.top + 1
  thread_stack[thread_stack.top] = thread
end


local function pop_thread()
  local thread = thread_top()
  thread_stack[thread_stack.top] = nil
  thread_stack.top = thread_stack.top - 1
  return thread
end


local function replay_pop()
  local frame = get_top()
  stack[stack.top] = nil
  stack.top = stack.top - 1
  return frame
end


local function pop()
  local frame = replay_pop()
  local thread = thread_top()
  if thread.top == 0 then
    pop_thread()
    thread = thread_top()
  end
  if thread then
    thread[thread.top] = nil
    thread.top = thread.top - 1
  end
  return frame
end


local function play_return(callee, caller)
  local time = total_time - caller.line_start_time
  local line = caller.current_line
  if line then
    time = time + caller.line_self_time_at_start - line.self_time
    time = time + caller.line_child_time_at_start - line.child_time
    line.child_time = line.child_time + time
  end

  local func = callee.func
  time = total_time - callee.func_start_time
  time = time + callee.func_self_time_at_start - func.self_time
  time = time + callee.func_child_time_at_start - func.child_time
  func.child_time = math.max(0, func.child_time + time)
end


local function do_return()
  while true do
    if stack.top <= 0 then
      error_count = error_count + 1
      local top = get_top()
      io.stderr:write(("ERROR (%4d, line %7d): tried to return above end of stack from function defined at %s:%d-%d\n"):
        format(error_count, trace_count, top and top.func.source_file.filename or "<no file>", top and top.line_defined or 0, top and top.last_line_defined or 0))
      break
    else
      local callee = pop()
      local caller = get_top()
      if caller then
        caller.protected = false
        play_return(callee, caller)
      end
      if not callee.tailcall then break end
    end
  end
end


function profile.record(a, b, c, d)
  trace_count = trace_count + 1

  if a == "S" or a == ">" or a == "T" then      -- Start, call or tailcall
    local filename, line_defined, last_line_defined = b, c, d
    local func = get_function(filename, line_defined, last_line_defined)
    local frame = { func=func, tailcall=(a=="T") or nil }
    call(frame)
    if not thread_top() then
      push_thread(get_thread(-1))
    end
    call_on_thread(frame)

  elseif a == "<" then                          -- Return
    do_return()

  elseif a == "R" then                          -- Resume
    local thread_id = b
    local thread = get_thread(thread_id)
    -- replay the thread onto the stack
    for _, frame in ipairs(thread) do
      call(frame)
    end
    push_thread(thread)

  elseif a == "Y" then                          -- Yield
    if thread_stack.top <= 0 then
      error_count = error_count + 1
      local top = get_top()
      io.stderr:write(("ERROR (%4d, line %7d): tried to yield to unknown thread from function defined at %s:%d-%d\n"):
        format(error_count, trace_count, top.func.source_file.filename, top.line_defined or 0, top.last_line_defined or 0))
    else
      local thread = thread_top()
      -- unwind the thread from the stack
      for i = thread.top, 1, -1 do
        local callee = replay_pop()
        local caller = get_top()
        thread[i].current_line = callee.current_line
        play_return(callee, caller)
      end
      pop_thread()
    end

  elseif a == "P" then                          -- pcall
    get_top().protected = true

  elseif a == "E" then                          -- Error!
    while true do
      local callee = pop()
      local caller = get_top()
      if not caller then break end
      play_return(callee, caller)
      if caller.protected then
        caller.protected = false
        break
      end
    end

  else                                          -- Line
    local line_number, time = a, b
    total_time = total_time + time

    local top = get_top()

    if not top then
      error_count = error_count + 1
      io.stderr:write(("ERROR (%4d, line %7d): recorded execution of %g microseconds at line %d in an unknown stack frame at height %d\n")
        :format(error_count, trace_count, time, line_number, stack.top))
    else
      if top.func.line_defined > 0 and
        (line_number < top.func.line_defined or line_number > top.func.last_line_defined) then
        -- luajit sometimes forgets to tell us about returns at all, so guess that
        -- there might have been one and try again
        local above_top = stack[stack.top-1]
        if not above_top or
          (above_top.func.line_defined > 0 and
           (line_number < above_top.func.line_defined or line_number > above_top.func.last_line_defined)) then
          error_count = error_count + 1
          io.stderr:write(("ERROR (%4d, line %7d): counted execution of %g microseconds at line %d of a function defined at %s:%d-%d\n"):
            format(error_count, trace_count, time, line_number, top.func.source_file.filename, top.func.line_defined, top.func.last_line_defined))
        else
          do_return()
          top = get_top()
        end
      end

      local line = get_line(line_number)
      if top.current_line ~= line then
        line.hits = line.hits + 1
      end
      line.self_time = line.self_time + time
      top.func.self_time = top.func.self_time + time
      top.current_line = line
    end
  end
end


-- Generating reports ----------------------------------------------------------

local function read_source()
  -- Read all the source files, and sort them into alphabetical order
  local max_line_length = 0
  local sorted_source_files = {}
  for _, f in pairs(source_files) do
    if not f.filename:match("l*uatrace") then
      local s = io.open(f.filename, "r")
      if s then
        sorted_source_files[#sorted_source_files+1] = f
        local i = 1
        for l in s:lines() do
          max_line_length = math.max(max_line_length, l:len())
          if f.lines[i] then
            f.lines[i].text = l
          else
            f.lines[i] = { text=l, file=f, line_number=i }
          end
          i = i + 1
        end
        s:close()
      end
    end
  end
  table.sort(sorted_source_files, function(a, b) return a.filename < b.filename end)

  return sorted_source_files, max_line_length
end


local function write_annotated_source(sorted_source_files, formats)
  local function source_format(f) return f.hit.." "..f.time.." "..f.time.." "..f.time.." "..f.line_number.." | %s" end
  local header_format = source_format(formats.s)
  local header = header_format:format("Hits", "Total", "Self", "Child", "Line", "")
  local line_format = source_format(formats.n)
  local asf = io.open("annotated-source.txt", "w")
  for _, f in ipairs(sorted_source_files) do
    asf:write(("="):rep(header:len() + formats.max_line_length), "\n")
    asf:write(header, f.filename, " - Times in ", formats.time_units, "\n")
    asf:write(("-"):rep(header:len() + formats.max_line_length), "\n")
    for i, l in ipairs(f.lines) do
      if f.functions[i] then
        for _, func in ipairs(f.functions[i]) do
          asf:write(line_format:format(func.hits, (func.self_time+func.child_time) / formats.divisor, func.self_time / formats.divisor, func.child_time / formats.divisor, i, "-- Function totals"), "\n")
        end
      end
      if l.hits then
        asf:write(line_format:format(l.hits, (l.self_time+l.child_time) / formats.divisor, l.self_time / formats.divisor, l.child_time / formats.divisor, i, l.text), "\n")
      else
        asf:write(header_format:format(".", ".", ".", ".", tonumber(l.line_number), l.text), "\n")
      end
    end
    asf:write("\n")
  end
  asf:close()
end


function profile.close()
  local formats = { n = {}, s = {}}
  local sorted_source_files

  sorted_source_files, formats.max_line_length = read_source()

  local all_lines = {}

  -- Work out the "most" of some numbers, so we can format reports better
  local max_time, max_hits, max_line_number = 0, 0, 0
  for _, l in ipairs(lines) do
    if l.child_time > total_time then
      error_count = error_count + 1
      io.stderr:write(("ERROR (%4d): Line %s:%d accumulated child execution time of %g microseconds which is more that the total running time of %g microseconds: %s\n"):
        format(error_count, l.file.filename, l.line_number, l.child_time, total_time, l.text or "-"))
    elseif l.child_time < 0 then
      error_count = error_count + 1
      io.stderr:write(("ERROR (%4d): Line %s:%d accumulated a negative child execution time (%g microseconds): %s\n"):
        format(error_count, l.file.filename, l.line_number, l.child_time, l.text or "-"))
    end

    max_time = math.max(max_time, l.self_time + l.child_time)
    max_hits = math.max(max_hits, l.hits)
    max_line_number = math.max(max_line_number, l.line_number)
  end

  local divisor, time_units, time_format
  if max_time < 10 then
    formats.divisor = 0.001
    formats.time_units = "nanoseconds"
    time_format = "d"
  elseif max_time < 10000 then
    formats.divisor = 1
    formats.time_units = "microseconds"
    time_format = ".2f"
  elseif max_time < 10000000 then
    formats.divisor = 1000
    formats.time_units = "milliseconds"
    time_format = ".2f"
  else
    formats.divisor = 1000000
    formats.time_units = "seconds"
    time_format = ".2f"
  end
  local function number_format(title, fmt, max_value)
    local f1 = "%"..fmt
    local max_len = f1:format(max_value):len()
    max_len = math.max(title:len(), max_len)
    local number_format = ("%%%d"..fmt):format(max_len)
    local string_format = ("%%%ds"):format(max_len)
    return number_format, string_format
  end

  formats.n.time, formats.s.time = number_format("Child", time_format, max_time / formats.divisor)
  formats.n.hit, formats.s.hit = number_format("Hits", "d", max_hits)
  formats.n.line_number, formats.s.line_number = number_format("line", "d", max_line_number)

  write_annotated_source(sorted_source_files, formats)

  -- Report on the lines using the most time
  table.sort(lines, function(a, b) return a.self_time + a.child_time > b.self_time + b.child_time end)
  local f2 = {}
  for _, f in pairs(functions) do f2[#f2+1] = f end
  table.sort(f2, function(a, b) return a.self_time > b.self_time end)

  local title_len = 9
  for i = 1, math.min(20, #lines) do
    local l = lines[i]
    l.title = ("%s:%d"):format(l.file.filename, l.line_number)
    title_len = math.max(title_len, l.title:len())
  end
  for i = 1, math.min(20, #f2) do
    local f = f2[i]
    f.title = ("%s:%d-%d"):format(f.source_file.filename, f.line_defined, f.last_line_defined)
    title_len = math.max(title_len, f.title:len())
  end
  local title_format = ("%%-%ds"):format(title_len)

  io.stderr:write(("Total time "..formats.n.time.." %s\n"):format(total_time / formats.divisor, formats.time_units))
  io.stderr:write("Times in ", formats.time_units, "\n")

  local function report_format(f) return title_format.."  "..f.hit.."  "..f.time.."  "..f.time.."  "..f.time.. " | %s\n" end
  line_format = report_format(formats.n)

  io.stderr:write("Top 20 lines by total time\n")
  io.stderr:write(report_format(formats.s):format("File:line", "Hits", "Total", "Self", "Child", "Line"))
  for i = 1, math.min(20, #lines) do
    local l = lines[i]
    io.stderr:write(line_format:format(l.title, l.hits,
      (l.self_time + l.child_time) / formats.divisor, l.self_time / formats.divisor, l.child_time / formats.divisor,
      l.text or "-"))
  end

  io.stderr:write("\nTop 20 functions by self time\n")
  io.stderr:write(report_format(formats.s):format("File:lines", "Hits", "Total", "Self", "Child", "Line"))
  for i = 1, math.min(20, #f2) do
    local l = f2[i]
    local line = l.source_file.lines[l.line_defined]
    l.text = line and line.text or nil
    io.stderr:write(line_format:format(l.title, l.hits,
      (l.self_time + l.child_time) / formats.divisor, l.self_time / formats.divisor, l.child_time / formats.divisor,
      l.text or "-"))
  end
end


-- Stopping and starting -------------------------------------------------------

function profile.go(trace_file_name)
  if trace_file_name == "" then trace_file_name = nil end
  trace_file.read{ recorder=profile, trace_file_name=trace_file_name }
end


local luatrace = require("luatrace")

function profile.tron(settings)
  settings = settings or {}
  settings.recorder = profile
  return luatrace.tron(settings)
end

profile.troff = luatrace.troff


-- Main ------------------------------------------------------------------------

if arg and type(arg) == "table" and string.match(debug.getinfo(1, "S").short_src, arg[0]) then
  profile.go()
end


--------------------------------------------------------------------------------

return profile


-- EOF -------------------------------------------------------------------------
