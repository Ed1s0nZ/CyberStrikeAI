---
name: recon-dir-scan
description: Directory and file enumeration using ffuf, gobuster, dirsearch, and feroxbuster. Use this skill when user needs to discover hidden directories, enumerate files, find backup files, or map application structure through path fuzzing.
---

# Directory and File Enumeration

## Authorization Warning

**IMPORTANT:** Directory scanning without proper authorization may be detected as intrusion attempts. Always ensure you have:
- Written permission from the target application owner
- Defined scope of authorized testing
- Legal compliance with local regulations

## Prerequisites

Required tools that must be installed on your system:
- **ffuf** - `go install github.com/ffuf/ffuf@latest`
- **gobuster** - `go install github.com/OJ/gobuster/v3/cmd/gobusterdir@main`

Optional tools:
- **feroxbuster** - `cargo install feroxbuster`
- **dirsearch** - `pip install dirsearch`
- **dirb** - Package manager installation

## Quick Start

Most commonly used commands for directory enumeration:

### Fast Directory Scan (ffuf)
```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ
```

### Recursive Scan with Status Codes
```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -mc 200,301,302 -recursion
```

### Gobuster Quick Scan
```bash
gobuster dir -u https://target.com -w wordlist.txt -t 50
```

## Common Scenarios

### Scenario 1: Basic Directory Fuzzing

When you need to discover directories and files:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ
```

**Parameters:**
- `-w wordlist.txt` - Wordlist path
- `-u` - Target URL with FUZZ keyword
- `-t` - Threads (default: 40)

**Example:**
```bash
ffuf -w /usr/share/seclists/Discovery/Web-Content/common.txt -u https://target.com/FUZZ
```

---

### Scenario 2: Filter by Status Code

When you only want specific HTTP status codes:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -mc 200,301,302,403
```

**Status Code Filters:**
| Code | Meaning |
|------|---------|
| 200 | OK (valid page) |
| 301,302 | Redirects |
| 403 | Forbidden (directory exists, no access) |
| 401 | Authentication required |

**Negation filter (exclude codes):**
```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -mc 200,204,301,302,307,401,403 -mc 404
```

---

### Scenario 3: Recursive Scanning

When you need to scan discovered directories recursively:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -recursion -recursion-depth 2
```

**Parameters:**
- `-recursion` - Enable recursive scanning
- `-recursion-depth` - Maximum depth (default: 0)

**Example:**
```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -recursion -recursion-depth 3
```

---

### Scenario 4: File Extension Fuzzing

When searching for specific file types:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -X .txt
```

**Multiple extensions:**
```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -X .txt,.php,.bak,.old
```

**Extension from wordlist:**
```bash
# Format: extension:wordlist
ffuf -w extensions.txt:EXT -w words.txt:FUZZ -u https://target.com/FUZZ.EXT
```

---

### Scenario 5: Hidden File Discovery

When looking for backup, config, or hidden files:

```bash
# Hidden dotfiles
ffuf -w hidden_files.txt -u https://target.com/FUZZ

# Backup files
ffuf -w wordlist.txt -u https://target.com/FUZZ -X .bak,.backup,.old,.tmp,.swp

# Config files
ffuf -w config_files.txt -u https://target.com/FUZZ
```

**Common hidden files to check:**
```
.git
.env
.env.local
.env.backup
config.php.bak
wp-config.php.bak
.gitignore
.htaccess
```

---

### Scenario 6: Virtual Host Discovery

When testing for virtual host routing:

```bash
ffuf -w vhosts.txt -u https://target.com -H "Host: FUZZ.target.com"
```

**Parameters:**
- `-H` - Add custom header
- `FUZZ` in header value - Replaced by wordlist entries

**Example:**
```bash
ffuf -w subdomains.txt -u http://192.168.1.100 -H "Host: FUZZ.example.com"
```

---

### Scenario 7: API Endpoint Discovery

When enumerating API endpoints:

```bash
ffuf -w api_endpoints.txt -u https://target.com/api/FUZZ
```

**Common API patterns:**
```
/api/v1/FUZZ
/api/v2/FUZZ
/graphql
/api/graphql
/rest/FUZZ
```

**With HTTP methods:**
```bash
ffuf -w endpoints.txt -u https://target.com/api/FUZZ -X GET,POST,PUT,DELETE
```

---

### Scenario 8: Parameter Fuzzing

When discovering hidden parameters:

```bash
ffuf -w params.txt -u https://target.com/page?FUZZ=test
```

**Value fuzzing:**
```bash
ffuf -w values.txt -u https://target.com/page?param=FUZZ
```

---

### Scenario 9: Gobuster Scanning

Alternative to ffuf using gobuster:

```bash
# Basic scan
gobuster dir -u https://target.com -w wordlist.txt -t 50

# With status filtering
gobuster dir -u https://target.com -w wordlist.txt -t 50 -k --status-codes 200,301,302,403

# Recursive
gobuster dir -u https://target.com -w wordlist.txt -t 50 -r

# With extensions
gobuster dir -u https://target.com -w wordlist.txt -t 50 -x php,txt,html
```

---

### Scenario 10: Feroxbuster Scanning

Modern Rust-based directory scanner:

```bash
# Basic scan
feroxbuster -u https://target.com -w wordlist.txt

# With recursion and status codes
feroxbuster -u https://target.com -w wordlist.txt -C 404 --depth 3

# Scan multiple URLs
feroxbuster -u https://target.com -u https://target2.com -w wordlist.txt
```

---

## Tool Selection Guide

| Scenario | Recommended Tool | Command |
|----------|------------------|---------|
| Quick scan | ffuf | `ffuf -w wordlist.txt -u https://target.com/FUZZ` |
| Recursive scan | ffuf | `ffuf -w wordlist.txt -u https://target.com/FUZZ -recursion` |
| Large wordlist | gobuster | `gobuster dir -u https://target.com -w wordlist.txt -t 100` |
| Multi-target | feroxbuster | `feroxbuster -u https://target.com -w wordlist.txt` |
| Hidden files | ffuf | `ffuf -w files.txt -u https://target.com/FUZZ` |
| API discovery | ffuf | `ffuf -w api.txt -u https://target.com/api/FUZZ` |

**Tool Comparison:**

| Tool | Language | Speed | Features | Best For |
|------|----------|-------|----------|----------|
| **ffuf** | Go | Very Fast | Highly flexible, filtering | Most scenarios |
| **gobuster** | Go | Fast | Simple, reliable | Quick scans |
| **feroxbuster** | Rust | Fast | Multi-target, recursion | Large assessments |
| **dirsearch** | Python | Medium | Built-in wordlists | Beginners |

---

## Common Wordlists

| Wordlist | Size | Description | Location |
|----------|------|-------------|----------|
| common.txt | ~4,600 | Common dirs/files | SecLists |
| raft-medium-directories | ~30,000 | Medium coverage | SecLists |
| raft-large-directories | ~60,000 | Large coverage | SecLists |
| directory-list-2.3-medium | ~220,000 | Comprehensive | DirBuster |
| apache.txt | ~5,000 | Apache defaults | SecLists |
| api-endpoints.txt | ~500 | API discovery | Custom |

**SecLists paths:**
```bash
/usr/share/seclists/Discovery/Web-Content/
/usr/share/seclists/Discovery/Web-Content/api/
/usr/share/seclists/Discovery/Web-Content/raft/
```

---

## Advanced Techniques

### Rate Limiting

Avoid detection by slowing requests:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -rate 100
```

### User-Agent Rotation

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -H "User-Agent: Mozilla/5.0 (compatible; Googlebot/2.1)"
```

### Authentication

```bash
# Basic auth
ffuf -w wordlist.txt -u https://user:pass@target.com/FUZZ

# Header-based auth
ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -H "Authorization: Bearer TOKEN"
```

### Cookie/Session Based

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -H "Cookie: session=YOUR_SESSION_TOKEN"
```

### Output Matching

Filter by response content:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -mr "admin"  # Match response containing "admin"

ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -ms 1520    # Match specific response size
```

---

## Output Formats

### Save Results

```bash
# ffuf JSON output
ffuf -w wordlist.txt -u https://target.com/FUZZ -o results.json

# ffuf plain output
ffuf -w wordlist.txt -u https://target.com/FUZZ -o results.txt

# Gobuster output
gobuster dir -u https://target.com -w wordlist.txt -o results.txt
```

### Resume Scanning

```bash
# ffuf resume
ffuf -w wordlist.txt -u https://target.com/FUZZ -resume-ffuf
```

---

## Tips and Best Practices

1. **Start small** - Use smaller wordlists first for quick wins
2. **Filter aggressively** - Use `-mc` to reduce noise
3. **Check response sizes** - Same size pages often indicate false positives
4. **Verify manually** - Always check interesting results manually
5. **Rate limiting** - Avoid blocking with appropriate delays
6. **Combine wordlists** - Multiple wordlists for better coverage
7. **Check parameters** - Don't forget to fuzz query parameters
8. **Look for 403s** - Forbidden responses reveal valid paths

---

## Resources

### Scripts

- `scripts/ffuf_results_parser.py` - Parse and filter ffuf JSON results
- `scripts/merge_wordlists.py` - Merge and deduplicate multiple wordlists
- `scripts/status_code_analyzer.py` - Analyze response patterns

### References

- `references/ffuf_guide.md` - Comprehensive ffuf reference
- `references/gobuster_guide.md` - Gobuster usage documentation
- `references/wordlist_guide.md` - Wordlist selection and creation


---

### Scenario: Persistent Storage of Directory Scan Findings

When you need to persist directory scan findings to the database:

```bash
# Manual entry after discovering directories
python .claude/skills/recon-dir-scan/scripts/dir_scan_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com" \
  --path "/admin" \
  --status 200 \
  --size 1234 \
  --subsystem "Web Application"
```

**Parameters:**
- `--host-ip` - Target host IP (required)
- `--url` - Base URL (required)
- `--path` - Discovered path (required)
- `--status` - HTTP status code (optional)
- `--size` - Response size (optional)
- `--tool` - Tool used (default: ffuf)
- `--subsystem` - Subsystem name (optional)

**Database location:** `./data/results.db`

**Related skills:** `results-storage` - Query data, generate reports

---
### Assets

- `assets/common-dirs.txt` - Common directory names
- `assets/common-files.txt` - Common file names
- `assets/hidden-files.txt` - Hidden and backup files
- `assets/api-endpoints.txt` - Common API endpoints
