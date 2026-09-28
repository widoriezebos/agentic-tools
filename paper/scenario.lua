-- The running example is marked in the source as blockquotes of italic text.
-- In the PDF a tinted panel carries that distinction (see style.tex), so the
-- italics are redundant here and are removed: a long italic passage is harder
-- to read than the same passage upright.
--
-- This strips emphasis only inside blockquotes, where the whole passage is
-- italic by convention. Emphasis in ordinary body text is untouched.

-- Consecutive blockquotes with no text between them are one passage broken
-- into paragraphs, not several passages. As separate panels they stack into a
-- run of boxes; merged, they read as one scene with its paragraphs intact.
function Blocks(blocks)
  local merged = pandoc.Blocks({})
  for _, block in ipairs(blocks) do
    local previous = merged[#merged]
    if block.t == "BlockQuote" and previous ~= nil and previous.t == "BlockQuote" then
      local combined = previous.content
      combined:extend(block.content)
      merged[#merged] = pandoc.BlockQuote(combined)
    else
      merged:insert(block)
    end
  end
  return merged
end

function BlockQuote(quote)
  return pandoc.walk_block(quote, {
    Emph = function(emphasis) return emphasis.content end,
  })
end
