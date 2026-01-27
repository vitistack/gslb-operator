-- Sandbox configuration for health check scripts
-- This defines what functions and libraries are available to user scripts

env = {
  -- Basic type operations
  type = type,
  tonumber = tonumber,
  tostring = tostring,
  
  -- Table iteration
  pairs = pairs,
  ipairs = ipairs,
  next = next,
  select = select,
  
  -- Error handling
  assert = assert,
  error = error,
  pcall = pcall,
  
  -- String operations
  string = {
    byte = string.byte,
    char = string.char,
    find = string.find,
    format = string.format,
    gmatch = string.gmatch,
    gsub = string.gsub,
    len = string.len,
    lower = string.lower,
    match = string.match,
    rep = string.rep,
    reverse = string.reverse,
    sub = string.sub,
    upper = string.upper,
  },
  
  -- Table manipulation
  table = {
    insert = table.insert,
    remove = table.remove,
    sort = table.sort,
    concat = table.concat,
  },
  
  -- Math operations
  math = {
    abs = math.abs,
    acos = math.acos,
    asin = math.asin,
    atan = math.atan,
    ceil = math.ceil,
    cos = math.cos,
    deg = math.deg,
    exp = math.exp,
    floor = math.floor,
    fmod = math.fmod,
    huge = math.huge,
    log = math.log,
    max = math.max,
    min = math.min,
    pi = math.pi,
    rad = math.rad,
    random = math.random,
    sin = math.sin,
    sqrt = math.sqrt,
    tan = math.tan,
  },
}
