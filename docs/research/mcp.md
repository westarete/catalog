# What Claude's documentation says about MCP

An MCP server could give agents the catalog without any file. An agent
would ask for the catalog for its current path and get what
`.catalog/catalog.md` would have held. This note records what
Anthropic's documentation says about how Claude Code and Cowork load and
reach MCP servers, which decides how well that could work. What the
catalog makes of it is in
[catalogs](../explanation/catalogs.md#delivering-the-catalog-without-a-file).

## What the documentation says

These points come from Claude Code's
[MCP documentation](https://code.claude.com/docs/en/mcp) unless we give
another source.

- Claude Code loads a server's instructions at the start of a session,
  cut off at 4,096 characters. It defers the server's tools by default
  and finds them on demand.
- Claude Code tells a server which folder the session is in. It answers
  the server's request for roots with the session's launch directory,
  and it sets `CLAUDE_PROJECT_DIR` for a server it starts.
- A team can commit a `.mcp.json` so that everyone who opens the
  repository gets the server. Each person approves it once in an
  interactive session. A plugin can also bundle a server.
- Cowork's connectors use MCP. A server that runs as a local command
  loads "when the Cowork session runs on your computer"
  ([plugin platform support](https://claude.com/docs/plugins/platform-support)).
  The documentation we read doesn't say whether that server runs on the
  Mac itself or inside Cowork's virtual machine.
- Anthropic's servers, not the person's machine, call a custom remote
  connector, so the connector has to be "reachable over the public
  internet from Anthropic's IP ranges"
  ([custom connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)).
