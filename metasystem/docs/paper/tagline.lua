-- Each chapter opens with a one-line deck: the paragraph straight after the
-- chapter heading, written in the source as a single bold run. Bold alone does
-- not carry it, so the PDF gives it its own treatment (see style.tex). The
-- source keeps the bold, which is what reads best on GitHub.
--
-- Only the paragraph immediately after a level-1 heading qualifies, and only
-- when it is entirely one bold run, so an ordinary bold sentence elsewhere in
-- the text is never caught.

local function is_deck(block)
  if block == nil or block.t ~= "Para" then return false end
  if #block.content ~= 1 then return false end
  return block.content[1].t == "Strong"
end

function Blocks(blocks)
  local result = pandoc.Blocks({})
  local after_heading = false

  for _, block in ipairs(blocks) do
    if after_heading and is_deck(block) then
      result:insert(pandoc.RawBlock("latex", "\\begin{chapterdeck}"))
      result:insert(pandoc.Para(block.content[1].content))
      result:insert(pandoc.RawBlock("latex", "\\end{chapterdeck}"))
    else
      result:insert(block)
    end
    after_heading = block.t == "Header" and block.level == 1
  end

  return result
end
