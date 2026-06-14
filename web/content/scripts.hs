-- Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
-- SPDX-License-Identifier: 	AGPL-3.0-or-later

-- GetQueryParam will fetch the value of the given query parameter. If no query parameter exists with the given name, an
-- empty string is returned.
def GetQueryParam(name)
  make a URLSearchParams from the search of the document's location called query
  if query.has(name)
    set value to the query.get(name)
    return value
  else
    return ''
  end
end
