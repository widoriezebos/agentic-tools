-- Give tables that carry no column widths an equal share of the text width.
--
-- Pipe tables in the source declare no widths, so LaTeX lays them out at their
-- natural width and long prose cells run off the page. Equal columns keep the
-- table inside the margins and let the cells wrap.

function Table(table_element)
  local columns = #table_element.colspecs
  if columns == 0 then return nil end

  for _, colspec in ipairs(table_element.colspecs) do
    if type(colspec[2]) == "number" then return nil end
  end

  for _, colspec in ipairs(table_element.colspecs) do
    colspec[2] = 1.0 / columns
  end
  return table_element
end
