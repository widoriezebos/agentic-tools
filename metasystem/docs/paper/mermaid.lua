-- Render mermaid code blocks to images so the appendix diagrams appear in the
-- PDF as diagrams. Without a renderer the block is left alone: the build still
-- succeeds and the diagram source stays readable in the output.
--
-- The Makefile supplies MMDC (a mermaid-cli binary), MERMAID_DIR (a build
-- directory), MERMAID_CONFIG (the shared mermaid theme) and MERMAID_LAYOUT
-- (per-diagram overrides). Images are cached under a name derived from the
-- diagram source and its effective config, so any change to either renders
-- again and a stale image can never survive.

local mmdc = os.getenv("MMDC") or ""
local build_dir = os.getenv("MERMAID_DIR") or "rendered/.build"
local base_config_path = os.getenv("MERMAID_CONFIG") or ""
local layout_path = os.getenv("MERMAID_LAYOUT") or ""

local counter = 0
local warned = false

local function succeeded(ok, _, code)
  -- Lua 5.1 returns a number, 5.2+ a boolean plus an exit code.
  if type(ok) == "number" then return ok == 0 end
  return ok == true and (code == nil or code == 0)
end

local function shell_quote(value)
  return "'" .. value:gsub("'", "'\\''") .. "'"
end

local function read_file(path)
  if path == "" then return nil end
  local handle = io.open(path, "r")
  if not handle then return nil end
  local contents = handle:read("a")
  handle:close()
  return contents
end

local function write_file(path, text)
  local handle = io.open(path, "w")
  if not handle then return false end
  handle:write(text)
  handle:close()
  return true
end

local function decode(text)
  if text == nil then return nil end
  local ok, value = pcall(pandoc.json.decode, text)
  if ok and type(value) == "table" then return value end
  return nil
end

local base_config = decode(read_file(base_config_path)) or {}
local layout = decode(read_file(layout_path)) or {}

-- The config a diagram is actually rendered with: the shared theme, plus this
-- diagram's spacing overrides if it has any.
local function effective_config(override)
  local config = decode(pandoc.json.encode(base_config)) or {}
  if override then
    config.flowchart = config.flowchart or {}
    if override.nodeSpacing then config.flowchart.nodeSpacing = override.nodeSpacing end
    if override.rankSpacing then config.flowchart.rankSpacing = override.rankSpacing end
  end
  return pandoc.json.encode(config)
end

function CodeBlock(block)
  if not block.classes:includes("mermaid") then return nil end
  counter = counter + 1

  if mmdc == "" then
    if not warned then
      io.stderr:write("mermaid: no renderer (MMDC unset); diagram source left in " ..
        "place. Run `make mermaid-cli` to render the diagrams.\n")
      warned = true
    end
    return nil
  end

  local override = layout[tostring(counter)]
  if type(override) ~= "table" then override = nil end

  -- A direction override rewrites only the diagram's first line.
  local text = block.text
  if override and override.direction then
    text = text:gsub("^(%s*%a+)%s+[LRTDB][RLTDB]", "%1 " .. override.direction, 1)
  end

  local config = effective_config(override)
  local stem = build_dir .. "/diagram-" .. counter .. "-" ..
    pandoc.utils.sha1(text .. "\0" .. config):sub(1, 12)
  local source_path = stem .. ".mmd"
  local config_path = stem .. ".json"
  local image_path = stem .. ".png"

  if not write_file(source_path, text) or not write_file(config_path, config) then
    io.stderr:write("mermaid: cannot write the source for diagram " .. counter .. "\n")
    return nil
  end

  if read_file(image_path) == nil then
    local command = table.concat({
      shell_quote(mmdc),
      "--input", shell_quote(source_path),
      "--output", shell_quote(image_path),
      "--configFile", shell_quote(config_path),
      "--backgroundColor white",
      "--scale 3",
      ">/dev/null 2>&1",
    }, " ")
    if not succeeded(os.execute(command)) then
      io.stderr:write("mermaid: render failed for diagram " .. counter ..
        "; leaving its source in place\n")
      return nil
    end
  end

  -- No explicit width: pandoc then bounds the image to both the text width and
  -- the text height, so a tall diagram cannot run off the page.
  return pandoc.Para({ pandoc.Image({}, image_path, "") })
end
