## mnemo

mnemo provides persistent memory for coding sessions via MCP tools (`mem_save`, `mem_search`, `mem_context`, `mem_session_summary`).

mnemo is active only in repositories initialized with `mnemo init` (a valid `.mnemo` file at the project root). If a mnemo tool returns an error indicating the project is not initialized, inform the user and suggest running `mnemo init` in the project directory. Do not retry the failed call.
