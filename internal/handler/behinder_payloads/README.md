# Behinder JSP Payload Sources

CyberStrikeAI's native Behinder protocol adapter needs the Java payload sources below when operating against authorized JSP Behinder webshells:

- `java/Cmd.java`
- `java/FileOperation.java`

At runtime CyberStrikeAI compiles these sources with `javac` into a temporary directory, reads the generated class bytes, rewrites selected static string fields in memory, encrypts the modified bytecode with the configured Behinder password, and sends it to the target webshell. Generated `.class` files are never checked into the repository.

These files live under `internal/handler` because they are private runtime assets for the WebShell handler, not user tool definitions. Do not place them under `tools/`, which is reserved for user-facing YAML tool recipes.

## Compatibility

The payloads intentionally keep the field names that the Go handler patches in memory:

- `Cmd`: `cmd`, `path`
- `FileOperation`: `mode`, `path`, `content`, `charset`, `newPath`

JSP Behinder support requires a JDK with `javac` available on `PATH` at runtime. PHP, ASP, and ASPX Behinder payload generation does not use these Java sources.
