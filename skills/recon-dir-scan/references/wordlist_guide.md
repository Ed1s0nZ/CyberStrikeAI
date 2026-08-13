# Wordlist Guide

Selecting and using wordlists for directory and file enumeration.

## Wordlist Categories

### Small / Quick Scans

| Wordlist | Entries | Source | Best For |
|----------|---------|--------|----------|
| common.txt | ~4,600 | SecLists | Quick checks |
| raft-small-words.txt | ~100 | SecLists | API parameters |
| Common.txt | ~1,000 | DirBuster | Fast discovery |

**Usage:**
```bash
ffuf -w common.txt -u https://target.com/FUZZ
```

### Medium Coverage

| Wordlist | Entries | Source | Best For |
|----------|---------|--------|----------|
| raft-medium-directories.txt | ~30,000 | SecLists | Standard scans |
| raft-medium-files.txt | ~50,000 | SecLists | File discovery |
| directory-list-2.3-small.txt | ~50,000 | DirBuster | Balanced scans |

**Usage:**
```bash
ffuf -w raft-medium-directories.txt -u https://target.com/FUZZ
```

### Large Coverage

| Wordlist | Entries | Source | Best For |
|----------|---------|--------|----------|
| raft-large-directories.txt | ~60,000 | SecLists | Comprehensive |
| directory-list-2.3-medium.txt | ~220,000 | DirBuster | Full enumeration |
| Commonspeak2-Wordlist.txt | ~380,000 | SecLists | Modern web |

**Usage:**
```bash
ffuf -w directory-list-2.3-medium.txt -u https://target.com/FUZZ -t 100
```

## SecLists Paths

Default location on most systems:
```bash
/usr/share/seclists/Discovery/Web-Content/
```

### Directory Structure

```
/usr/share/seclists/Discovery/Web-Content/
├── common.txt                          # Common files/dirs
├── raft/
│   ├── medium-directories.txt         # Medium directory list
│   ├── large-directories.txt          # Large directory list
│   ├── medium-files.txt               # Medium file list
│   └── large-files.txt                # Large file list
├── api/
│   └── api-endpoints.txt              # API endpoints
├── backup/
│   └── backup-files.txt               # Backup files
├── Attacks/
│   └── ...
├── CMS/
│   └── ...
└── ...
```

## Specialized Wordlists

### API Discovery

```bash
# API endpoints
ffuf -w /usr/share/seclists/Discovery/Web-Content/api/api-endpoints.txt \
  -u https://target.com/api/FUZZ

# GraphQL
ffuf -w graphql.txt -u https://target.com/FUZZ
```

### Backup Files

```bash
ffuf -w /usr/share/seclists/Discovery/Web-Content/backup-files.txt \
  -u https://target.com/FUZZ -X .bak,.old,.backup,.tmp
```

### Config Files

```bash
ffuf -w config_files.txt -u https://target.com/FUZZ
```

**Common config files:**
```
config.php
config.ini
.env
.env.local
web.config
application.properties
settings.py
```

### Hidden Files

```bash
ffuf -w hidden_files.txt -u https://target.com/FUZZ
```

**Common hidden files:**
```
.git
.gitignore
.env
.htaccess
.svn
.DS_Store
```

## Technology-Specific Wordlists

### WordPress

```bash
ffuf -w wordpress.txt -u https://target.com/FUZZ
```

**Common WordPress paths:**
```
wp-admin
wp-content
wp-includes
wp-login.php
xmlrpc.php
wp-config.php
```

### Laravel

```bash
ffuf -w laravel.txt -u https://target.com/FUZZ
```

### ASP.NET

```bash
ffuf -w aspnet.txt -u https://target.com/FUZZ
```

## Creating Custom Wordlists

### From Application

Extract words from JavaScript files:
```bash
curl -s https://target.com/app.js | \
  grep -oP '"[^/"]*\.[a-z]{2,4}"' | \
  sort -u > js_words.txt
```

### From Source Code

```bash
# Extract routes from source
grep -r "router\|route" source/ | \
  grep -oP "'[^']*'" | \
  sort -u > routes.txt
```

### Merge Multiple Wordlists

```bash
# Using provided script
python merge_wordlists.py list1.txt list2.txt list3.txt -o merged.txt

# Manual merge
cat list1.txt list2.txt | sort -u > merged.txt
```

## Wordlist Selection Strategy

1. **Start small** - Use common.txt for quick discovery
2. **Go medium** - Switch to raft-medium for standard assessment
3. **Targeted** - Use specialized lists for specific technologies
4. **Go large** - Use full lists for comprehensive assessment
5. **Custom** - Build custom lists based on target

## Tips

1. **Remove comments** - Clean wordlists improve performance
2. **Deduplicate** - Remove duplicates for efficiency
3. **Case sensitivity** - Most web servers are case-insensitive
4. **Language-specific** - Use localized wordlists for foreign targets
5. **Update regularly** - Wordlists evolve with web technologies
