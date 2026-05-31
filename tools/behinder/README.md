# Behinder Payload Assets

CyberStrikeAI's native Behinder protocol adapter needs the Java payload bytecode below when operating against authorized JSP Behinder webshells:

- `net/rebeyond/behinder/payload/java/Cmd.class`
- `net/rebeyond/behinder/payload/java/FileOperation.class`

At runtime CyberStrikeAI reads these class files, rewrites selected static string fields in memory, encrypts the modified bytecode with the configured Behinder password, and sends it to the target webshell. The files are kept in the repository so source deployments do not depend on `/opt/CyberStrikeAI` or a machine-specific absolute path.

## Origin and compatibility

The bytecode layout and field names are compatible with the public Behinder/rebeyond payload format. Reviewers can compare behavior with the upstream project release line:

- https://github.com/rebeyond/Behinder/releases

Current file fingerprints:

```text
8fbd565374dbdde5bd23273ce9c5778767c4522e2a7a21242aaecb2f242fab2f  Cmd.class
69ceaa1c31651a1e2cbb1cb2fe711839c28f6ebab48e4b0804d4727ee9549c5a  FileOperation.class
```

Both files are Java 8 class files (`file` reports version 52.0). If these payloads are regenerated or replaced, update the hashes in this document and verify the upstream license/redistribution terms before merging.
