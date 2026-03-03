---
name: api-security-testing
description: Professional skills and methodology for API security testing
version: 1.0.0
---

# API Security Testing

## Overview

API security testing is an essential part of ensuring the security of API interfaces. This skill provides methods, tools, and best practices for API security testing.

## Testing Scope

### 1. Authentication and Authorization

**Test items:**
- Token validity verification
- Token expiration handling
- Permission control
- Role permission verification

### 2. Input Validation

**Test items:**
- Parameter type validation
- Data length limits
- Special character handling
- SQL injection protection
- XSS protection

### 3. Business Logic

**Test items:**
- Workflow validation
- State transitions
- Concurrency control
- Business rules

### 4. Error Handling

**Test items:**
- Error message leakage
- Stack traces
- Sensitive information exposure

## Testing Methods

### 1. API Discovery

**Identify API endpoints:**
```bash
# Use directory scanning
gobuster dir -u https://target.com -w api-wordlist.txt

# Use Burp Suite passive scanning
# Browse application, observe API calls

# Analyze JavaScript files
# Find API endpoint definitions
```

### 2. Authentication Testing

**Token testing:**
```http
# Test invalid Token
GET /api/user
Authorization: Bearer invalid_token

# Test expired Token
GET /api/user
Authorization: Bearer expired_token

# Test without Token
GET /api/user
```

**JWT testing:**
```bash
# Use jwt_tool
python jwt_tool.py <JWT_TOKEN>

# Test algorithm confusion
python jwt_tool.py <JWT_TOKEN> -X a

# Test key brute force
python jwt_tool.py <JWT_TOKEN> -C -d wordlist.txt
```

### 3. Authorization Testing

**Horizontal privileges:**
```http
# User A accessing User B's resources
GET /api/user/123
Authorization: Bearer user_a_token

# Should return 403
```

**Vertical privileges:**
```http
# Regular user accessing admin interface
GET /api/admin/users
Authorization: Bearer user_token

# Should return 403
```

### 4. Input Validation Testing

**SQL injection:**
```http
POST /api/search
{
  "query": "test' OR '1'='1"
}
```

**Command injection:**
```http
POST /api/execute
{
  "command": "ping; id"
}
```

**XXE:**
```http
POST /api/parse
Content-Type: application/xml

<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<foo>&xxe;</foo>
```

### 5. Rate Limit Testing

**Test rate limits:**
```python
import requests

for i in range(1000):
    response = requests.get('https://target.com/api/endpoint')
    print(f"Request {i}: {response.status_code}")
```

## Tool Usage

### Postman

**Create test collection:**
1. Import API documentation
2. Set authentication
3. Create test cases
4. Run automated tests

### Burp Suite

**API scanning:**
1. Configure API endpoints
2. Set authentication
3. Run active scanning
4. Analyze results

### OWASP ZAP

```bash
# API scan
zap-cli quick-scan --self-contained \
  --start-options '-config api.disablekey=true' \
  http://target.com/api
```

### REST-Attacker

```bash
# Scan OpenAPI specification
rest-attacker scan openapi.yaml
```

## Common Vulnerabilities

### 1. Authentication Bypass

**Token validation flaws:**
- Weak Token generation
- Predictable Token
- Token signature not verified

### 2. Privilege Escalation

**IDOR:**
- Direct object references
- Resource ownership not verified

### 3. Information Disclosure

**Error messages:**
- Detailed error messages
- Stack traces
- Sensitive data

### 4. Injection Vulnerabilities

**Common injections:**
- SQL injection
- NoSQL injection
- Command injection
- XXE

### 5. Business Logic

**Logic flaws:**
- Price manipulation
- Quantity limit bypass
- State modification

## Testing Checklist

### Authentication Testing
- [ ] Token validity verification
- [ ] Token expiration handling
- [ ] Weak Token detection
- [ ] Token replay attack

### Authorization Testing
- [ ] Horizontal privilege testing
- [ ] Vertical privilege testing
- [ ] Role permission verification
- [ ] Resource access control

### Input Validation
- [ ] SQL injection testing
- [ ] XSS testing
- [ ] Command injection testing
- [ ] XXE testing
- [ ] Parameter pollution

### Business Logic
- [ ] Workflow validation
- [ ] State transitions
- [ ] Concurrency control
- [ ] Business rules

### Error Handling
- [ ] Error message leakage
- [ ] Stack traces
- [ ] Sensitive information exposure

## Protective Measures

### Recommended Solutions

1. **Authentication**
   - Use strong Tokens
   - Implement Token refresh
   - Verify Token signature

2. **Authorization**
   - Role-based access control
   - Resource ownership verification
   - Principle of least privilege

3. **Input Validation**
   - Parameter type validation
   - Data length limits
   - Whitelist validation

4. **Error Handling**
   - Unified error responses
   - Do not expose detailed information
   - Record error logs

5. **Rate Limiting**
   - Implement API rate limiting
   - Prevent brute force attacks
   - Monitor abnormal requests

## Notes

- Only perform testing in authorized test environments
- Avoid impacting APIs
- Note differences across API versions
- Pay attention to request frequency during testing
