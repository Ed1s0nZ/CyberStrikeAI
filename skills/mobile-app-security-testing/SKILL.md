---
name: mobile-app-security-testing
description: Professional skills and methodology for mobile application security testing
version: 1.0.0
---

# Mobile Application Security Testing

## Overview

Mobile application security testing is an essential part of ensuring the security of mobile applications. This skill provides methods, tools, and best practices for mobile application security testing, covering both Android and iOS platforms.

## Testing Scope

### 1. Application Security

**Checklist:**
- Code obfuscation
- Decompilation protection
- Debug protection
- Certificate pinning

### 2. Data Security

**Checklist:**
- Data encryption
- Key management
- Sensitive data storage
- Data transmission

### 3. Authentication and Authorization

**Checklist:**
- Authentication mechanisms
- Token management
- Biometric authentication
- Session management

### 4. Communication Security

**Checklist:**
- TLS/SSL configuration
- Certificate validation
- API security
- Man-in-the-middle attack protection

## Android Security Testing

### Static Analysis

**Using APKTool:**
```bash
# Decompile APK
apktool d app.apk

# View AndroidManifest.xml
cat app/AndroidManifest.xml

# View Smali code
find app/smali -name "*.smali"
```

**Using Jadx:**
```bash
# Decompile APK
jadx -d output app.apk

# View Java source code
find output -name "*.java"
```

**Using MobSF:**
```bash
# Start MobSF
docker run -it -p 8000:8000 opensecurity/mobsf

# Upload APK for analysis
# Visit http://localhost:8000
```

### Dynamic Analysis

**Using Frida:**
```javascript
// Hook function
Java.perform(function() {
    var MainActivity = Java.use("com.example.MainActivity");
    MainActivity.onCreate.implementation = function(savedInstanceState) {
        console.log("[*] onCreate called");
        this.onCreate(savedInstanceState);
    };
});
```

**Using Objection:**
```bash
# Start Objection
objection -g com.example.app explore

# Hook function
android hooking watch class_method com.example.MainActivity.onCreate
```

**Using Burp Suite:**
```bash
# Configure proxy
# Set Android proxy to point to Burp Suite
# Install Burp certificate
```

### Common Vulnerabilities

**Hardcoded keys:**
```java
// Insecure code
String apiKey = "1234567890abcdef";
String password = "admin123";
```

**Insecure storage:**
```java
// Storing sensitive data in SharedPreferences
SharedPreferences prefs = getSharedPreferences("data", MODE_WORLD_READABLE);
prefs.edit().putString("password", password).apply();
```

**Certificate validation bypass:**
```java
// Not validating certificates
TrustManager[] trustAllCerts = new TrustManager[] {
    new X509TrustManager() {
        public X509Certificate[] getAcceptedIssuers() { return null; }
        public void checkClientTrusted(X509Certificate[] certs, String authType) { }
        public void checkServerTrusted(X509Certificate[] certs, String authType) { }
    }
};
```

## iOS Security Testing

### Static Analysis

**Using class-dump:**
```bash
# Export header files
class-dump app.ipa

# View header files
find app -name "*.h"
```

**Using Hopper:**
```bash
# Use Hopper to disassemble
# Open app binary file
# Analyze assembly code
```

**Using otool:**
```bash
# View Mach-O information
otool -L app

# View strings
strings app | grep -i "password\|key\|secret"
```

### Dynamic Analysis

**Using Frida:**
```javascript
// Hook Objective-C method
var className = ObjC.classes.ViewController;
var method = className['- login:password:'];
Interceptor.attach(method.implementation, {
    onEnter: function(args) {
        console.log("[*] Login called");
        console.log("Username: " + ObjC.Object(args[2]).toString());
        console.log("Password: " + ObjC.Object(args[3]).toString());
    }
});
```

**Using Cycript:**
```bash
# Attach to process
cycript -p app

# Execute commands
[UIApplication sharedApplication]
```

### Common Vulnerabilities

**Hardcoded keys:**
```objective-c
// Insecure code
NSString *apiKey = @"1234567890abcdef";
NSString *password = @"admin123";
```

**Insecure storage:**
```objective-c
// Improper Keychain storage
NSUserDefaults *defaults = [NSUserDefaults standardUserDefaults];
[defaults setObject:password forKey:@"password"];
```

**Certificate validation bypass:**
```objective-c
// Not validating certificates
- (void)connection:(NSURLConnection *)connection
didReceiveAuthenticationChallenge:(NSURLAuthenticationChallenge *)challenge {
    [challenge.sender useCredential:[NSURLCredential credentialForTrust:challenge.protectionSpace.serverTrust]
          forAuthenticationChallenge:challenge];
}
```

## Tool Usage

### MobSF

```bash
# Start MobSF
docker run -it -p 8000:8000 opensecurity/mobsf

# Upload application for analysis
# Supports Android and iOS
```

### Frida

```bash
# Install Frida
pip install frida-tools

# Run script
frida -U -f com.example.app -l script.js
```

### Objection

```bash
# Install Objection
pip install objection

# Start Objection
objection -g com.example.app explore
```

### Burp Suite

**Configure proxy:**
1. Configure Burp Suite listener
2. Set proxy on mobile device
3. Install Burp certificate
4. Intercept and analyze traffic

## Testing Checklist

### Application Security
- [ ] Code obfuscation check
- [ ] Decompilation protection
- [ ] Debug protection
- [ ] Certificate pinning

### Data Security
- [ ] Data encryption check
- [ ] Key management
- [ ] Sensitive data storage
- [ ] Data transmission security

### Authentication and Authorization
- [ ] Authentication mechanism testing
- [ ] Token management
- [ ] Session management
- [ ] Biometric authentication

### Communication Security
- [ ] TLS/SSL configuration
- [ ] Certificate validation
- [ ] API security testing
- [ ] Man-in-the-middle attack protection

## Common Security Issues

### 1. Hardcoded Keys

**Issue:**
- API keys hardcoded
- Passwords hardcoded
- Encryption keys hardcoded

**Remediation:**
- Use key management services
- Use environment variables
- Use secure storage

### 2. Insecure Storage

**Issue:**
- Sensitive data stored in plaintext
- Using insecure storage methods
- Data not encrypted

**Remediation:**
- Use encrypted storage
- Use Keychain/Keystore
- Implement data encryption

### 3. Certificate Validation Bypass

**Issue:**
- SSL certificates not validated
- Self-signed certificates accepted
- Certificate pinning not implemented

**Remediation:**
- Implement certificate pinning
- Validate certificate chain
- Use system certificate store

### 4. Debug Information Leakage

**Issue:**
- Logs contain sensitive information
- Error messages leaked
- Debug mode not disabled

**Remediation:**
- Remove debug code
- Restrict log output
- Disable debug mode in production

## Best Practices

### 1. Code Security

- Implement code obfuscation
- Disable debug features
- Implement anti-debugging protection
- Use certificate pinning

### 2. Data Security

- Encrypt sensitive data
- Use secure storage
- Implement key management
- Restrict data access

### 3. Communication Security

- Use TLS/SSL
- Implement certificate pinning
- Validate server certificates
- Use secure APIs

### 4. Authentication Security

- Implement strong authentication
- Secure Token management
- Implement session management
- Use biometric authentication

## Notes

- Only perform testing in authorized environments
- Comply with laws and regulations
- Note differences across platforms
- Protect user privacy
