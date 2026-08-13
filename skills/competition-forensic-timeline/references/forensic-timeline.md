# Forensic Timeline Checklist

## Anchor Selection

- Earliest trustworthy markers: process creation, service install, scheduled task, mailbox rule write, browser download, network connect, or credential replay
- Note the time source for each artifact: event log local time, UTC network time, filesystem timestamp, mailbox server time, or packet capture timestamp
- Record drift, missing zones, daylight-saving ambiguity, or truncation before merging sources

## Cross-Source Correlation

Match on shared identifiers whenever possible:

1. Process ID, parent process ID, or command line
2. Logon ID, SID, ticket cache, mailbox item ID, or message trace ID
3. Hostname, IP, MAC, session ID, GUID, or hash
4. File path, registry path, service name, task name, or attachment name

## Timeline Compression

- Keep one long-form raw chronology for yourself and one short decisive chronology for the final answer
- Separate observed events from inferred transitions
- If an edge is inferred, state the missing validation step explicitly

## Common Pitfalls

- Sorting by timestamp alone when clock drift or delayed logging exists
- Mixing mailbox, browser, host, and network artifacts without shared identifiers
- Reporting a long event dump instead of the smallest sequence that proves the challenge path
