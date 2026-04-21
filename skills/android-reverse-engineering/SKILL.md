---
name: android-reverse-engineering
description: Deep Android APK/DEX reverse engineering - decompilation, API extraction, call flow tracing, Ghidra analysis, and dynamic testing with Cuttlefish VM
version: 1.0.0
---

# Android Reverse Engineering

## Overview

Comprehensive Android application reverse engineering skill covering static analysis (decompilation, Ghidra binary analysis, API extraction) and dynamic analysis (Cuttlefish VM, DroidRun UI automation, Frida hooking, traffic interception). Integrates with CyberStrikeAI's full tool chain.

## Phase 1: Dependency Setup

Before starting, verify tools are available. Run the dependency checker:

```bash
# Check what's installed
bash skills/android-reverse-engineering/scripts/check-deps.sh

# Install missing deps (auto-detects OS, installs user-locally when possible)
bash skills/android-reverse-engineering/scripts/install-dep.sh jadx
bash skills/android-reverse-engineering/scripts/install-dep.sh vineflower
bash skills/android-reverse-engineering/scripts/install-dep.sh dex2jar
```

### Required Tools
- **Java JDK 17+** - runtime for all decompilers and Ghidra
- **jadx** - primary DEX-to-Java decompiler (handles APK/AAR/JAR natively)

### Recommended Tools
- **Fernflower/Vineflower** - alternative decompiler (better on some obfuscated code)
- **dex2jar** - DEX-to-JAR converter (needed for Fernflower on APK files)
- **apktool** - resource decoder (layouts, strings, assets)
- **Ghidra Headless MCP** - deep binary analysis (native code, .so files, complex DEX)
- **Cuttlefish VM** - Android VM for dynamic analysis
- **Frida** - dynamic instrumentation

## Phase 2: Decompilation

### Quick Decompile (jadx)
```bash
# Basic decompilation
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk

# With deobfuscation
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk --deobf

# Skip resources (faster, source only)
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk --no-res

# Custom output directory
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk -o /tmp/decompiled
```

### Multi-Engine Decompile
```bash
# Use Fernflower/Vineflower
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk --engine fernflower

# Both engines for comparison (useful for obfuscated apps)
bash skills/android-reverse-engineering/scripts/decompile.sh target.apk --engine both
```

### XAPK Support
The decompile script handles XAPK bundles (ZIP containing multiple APKs):
- Auto-extracts the archive
- Finds all APK files inside
- Decompiles each into separate subdirectories
- Preserves manifest.json metadata

### Using CyberStrikeAI Tools
```
# Via exec tool:
exec command="jadx -d /tmp/decompiled --deobf target.apk"

# Or use the decompile script:
exec command="bash skills/android-reverse-engineering/scripts/decompile.sh target.apk --deobf -o /tmp/decompiled"
```

### Deep Analysis with Ghidra Headless MCP
For native libraries (.so files) or complex DEX analysis, use the Ghidra Headless MCP:
```
# If ghidra-headless-mcp external MCP is enabled:
program.open file_path="/path/to/target.apk"
analysis.update_and_wait session_id="..."
function.list session_id="..."
decomp.function session_id="..." name="com.target.crypto.AESHelper.encrypt"
search.defined_strings session_id="..."
```

For native .so libraries extracted from the APK:
```
program.open file_path="/tmp/decompiled/lib/arm64-v8a/libnative.so"
analysis.update_and_wait session_id="..."
external.imports.list session_id="..."
decomp.function session_id="..." name="Java_com_target_NativeLib_decrypt"
```

## Phase 3: Structure Analysis

### AndroidManifest.xml
Read the manifest to identify entry points:
```
# Key elements to find:
- <activity> - UI screens, especially android:exported="true" and intent-filters
- <service> - background services (data sync, push notifications)
- <receiver> - broadcast receivers (triggers, events)
- <provider> - content providers (data access)
- <uses-permission> - required permissions (INTERNET, READ_CONTACTS, CAMERA, etc.)
- <application android:name="..."> - Application class (initialization, DI setup)
- android:debuggable="true" - debug build leaked
- android:allowBackup="true" - backup vulnerability
- android:networkSecurityConfig - certificate pinning config
```

### Package Structure Survey
Look for these packages in the decompiled source:
- `api/`, `network/`, `service/` - HTTP clients and API definitions
- `data/`, `repository/`, `db/` - data layer, local storage
- `crypto/`, `security/`, `encryption/` - cryptographic operations
- `auth/`, `login/` - authentication logic
- `di/`, `module/`, `inject/` - dependency injection (Dagger/Hilt)
- `util/`, `helper/`, `common/` - utility classes (often contain crypto helpers)
- `model/`, `entity/`, `dto/` - data models (reveal API structure)

### Architecture Pattern Identification
- **MVP**: Presenter classes, Contract interfaces
- **MVVM**: ViewModel classes, LiveData/StateFlow usage
- **Clean Architecture**: UseCases, Domain/Data/Presentation layers
- **MVI**: Intent/State/Effect pattern

## Phase 4: Call Flow Tracing

### Entry Point → Network Call Chain
Typical Android call flow:
```
Activity.onCreate()
  → setContentView(R.layout.activity_login)
  → findViewById(R.id.loginButton).setOnClickListener()
    → ViewModel.login(username, password)
      → Repository.authenticate(credentials)
        → ApiService.login(@Body LoginRequest)
          → Retrofit → OkHttp → HTTP POST /api/v1/auth/login
```

### Finding Click Handlers
Search for UI interaction entry points:
```bash
# onClick handlers
grep -r "setOnClickListener\|onClick" /tmp/decompiled/sources/ --include="*.java"

# Activity lifecycle
grep -r "onCreate\|onResume\|onStart" /tmp/decompiled/sources/ --include="*.java" | grep -v "test\|Test"

# Fragment lifecycle
grep -r "onCreateView\|onViewCreated" /tmp/decompiled/sources/ --include="*.java"
```

### DI Tracing (Dagger/Hilt)
```bash
# Find DI modules
grep -r "@Module\|@Provides\|@Binds\|@Inject\|@Singleton" /tmp/decompiled/sources/ --include="*.java"

# Find the API service provider
grep -r "Retrofit.Builder\|OkHttpClient.Builder" /tmp/decompiled/sources/ --include="*.java"

# Find base URL
grep -r "baseUrl\|BASE_URL\|base_url\|ApiConfig\|BuildConfig" /tmp/decompiled/sources/ --include="*.java"
```

### Handling Obfuscated Code
What gets obfuscated (ProGuard/R8):
- Class names → `a.b.c`
- Method names → `a()`, `b()`
- Field names → `a`, `b`

What does NOT get obfuscated:
- String literals (URLs, error messages, format strings)
- Android framework classes (`Activity`, `SharedPreferences`, `Intent`)
- Library public APIs (`Retrofit`, `OkHttpClient`, `Room`)
- Manifest entries (activities, services, receivers)
- Annotation values (`@GET("/api/v1/users")`, `@SerializedName("email")`)

Strategy: Start from strings/URLs → follow framework method calls → trace backwards to find the obfuscated wrappers.

## Phase 5: API Extraction

### Automated API Discovery
```bash
# Find all API patterns
bash skills/android-reverse-engineering/scripts/find-api-calls.sh /tmp/decompiled/sources/

# Specific categories
bash skills/android-reverse-engineering/scripts/find-api-calls.sh /tmp/decompiled/sources/ --retrofit
bash skills/android-reverse-engineering/scripts/find-api-calls.sh /tmp/decompiled/sources/ --okhttp
bash skills/android-reverse-engineering/scripts/find-api-calls.sh /tmp/decompiled/sources/ --urls
bash skills/android-reverse-engineering/scripts/find-api-calls.sh /tmp/decompiled/sources/ --auth
```

### Pattern Reference

**Retrofit (most common)**:
```java
@GET("/api/v1/users/{id}")
Call<User> getUser(@Path("id") String userId, @Header("Authorization") String token);

@POST("/api/v1/auth/login")
Call<AuthResponse> login(@Body LoginRequest request);

@PUT("/api/v1/profile")
@Multipart
Call<Void> updateProfile(@Part MultipartBody.Part photo, @Part("name") RequestBody name);
```

**OkHttp**:
```java
Request request = new Request.Builder()
    .url(BASE_URL + "/api/v1/data")
    .addHeader("Authorization", "Bearer " + token)
    .post(RequestBody.create(json, MediaType.parse("application/json")))
    .build();
```

**Hardcoded Secrets**:
```bash
# Search for API keys, tokens, secrets
grep -rEi "(api[_-]?key|secret|token|password|auth)\s*[:=]" /tmp/decompiled/sources/ --include="*.java"

# Search for URLs
grep -rE "https?://[^\s\"')>]+" /tmp/decompiled/sources/ --include="*.java"

# Search for base64-encoded data
grep -rE "[A-Za-z0-9+/]{40,}={0,2}" /tmp/decompiled/sources/ --include="*.java"
```

### Endpoint Documentation Template
For each discovered endpoint, document:
```
Endpoint: POST /api/v1/auth/login
Base URL: https://api.target.com
Headers: Content-Type: application/json, X-App-Version: 2.1.0
Request Body: {"email": "string", "password": "string", "device_id": "string"}
Response: {"token": "string", "user_id": "int", "expires_at": "timestamp"}
Auth: None (login endpoint)
Call Chain: LoginActivity → LoginViewModel → AuthRepository → AuthApiService.login()
Source File: com/target/api/AuthApiService.java:45
```

## Phase 6: Dynamic Analysis with CyberStrikeAI

### Full Dynamic Analysis Chain
```
1. cuttlefish_launch                    → start Android VM
2. cuttlefish_install_apk apk_path=...  → install the target APK
3. cuttlefish_snapshot save clean        → save clean state
4. droidrun_connect                      → verify DroidRun proxy
5. droidrun_open_app package_name=...    → launch the app
6. droidrun_state                        → observe UI, read elements
7. droidrun_type text="..." index=N      → enter credentials
8. droidrun_click index=N                → tap buttons
9. cuttlefish_logcat filter="tag:OkHttp" → monitor network calls
```

### Frida Hooking (Guided by Static Analysis)
After finding interesting functions in Ghidra/jadx:
```
1. cuttlefish_frida_setup                → deploy Frida server
2. Use exec tool to run Frida scripts:

# Hook encryption function found in static analysis
frida -U -f com.target.app -l hook_crypto.js

# Example hook script (write via create-file tool):
Java.perform(function() {
    var AESHelper = Java.use("com.target.crypto.AESHelper");
    AESHelper.encrypt.implementation = function(data, key) {
        console.log("[*] AESHelper.encrypt called");
        console.log("    data: " + data);
        console.log("    key: " + key);
        var result = this.encrypt(data, key);
        console.log("    result: " + result);
        return result;
    };
});
```

### Traffic Interception
```
1. Start sslstrip or mitmproxy on host
2. cuttlefish_proxy set <host_ip> <port>   → route traffic through proxy
3. cuttlefish_install_cert cert=burp.pem   → install CA cert
4. droidrun_open_app → interact with app
5. Analyze captured traffic for credentials, tokens, API calls
```

### Certificate Pinning Bypass
```
1. cuttlefish_frida_setup → deploy Frida
2. Run universal SSL pinning bypass:
   frida -U -f com.target.app -l ssl_bypass.js

# Universal bypass script:
Java.perform(function() {
    var X509TrustManager = Java.use('javax.net.ssl.X509TrustManager');
    var SSLContext = Java.use('javax.net.ssl.SSLContext');

    var TrustManager = Java.registerClass({
        name: 'com.bypass.TrustManager',
        implements: [X509TrustManager],
        methods: {
            checkClientTrusted: function(chain, authType) {},
            checkServerTrusted: function(chain, authType) {},
            getAcceptedIssuers: function() { return []; }
        }
    });

    var TrustManagers = [TrustManager.$new()];
    var sslContext = SSLContext.getInstance("TLS");
    sslContext.init(null, TrustManagers, null);
});
```

## Combined Workflow: Static + Dynamic

The most effective approach combines both:

```
STATIC ANALYSIS (understand the code):
1. Decompile APK with jadx (--deobf)
2. Read AndroidManifest.xml - find entry points, permissions
3. Find API services - grep for Retrofit/OkHttp patterns
4. Decompile crypto functions with Ghidra Headless MCP
5. Map the call flow: UI → ViewModel → Repository → API → HTTP

DYNAMIC ANALYSIS (observe runtime behavior):
6. Launch Cuttlefish VM, install APK
7. Use DroidRun to navigate the app UI
8. Hook functions found in static analysis with Frida
9. Intercept traffic with proxy/SSLStrip
10. Compare: static findings vs actual runtime behavior

CROSS-REFERENCE:
- URLs found in decompiled code → verify in traffic capture
- Encryption functions in Ghidra → verify keys via Frida hooks
- API endpoints from Retrofit annotations → test with actual requests
- Hardcoded secrets in source → validate they're actually used at runtime
```

## Phase 7: Artifact Management and Process Memory

### IMPORTANT: Every step of the RE process MUST be tracked and persisted.

### Register the APK on Receipt
As soon as an APK is received or uploaded, register it with the File Manager:
```
register_file
  file_path: "/path/to/target.apk"
  file_type: "apk"
  summary: "Target Android APK - <app name>, <package name>, <version if known>"
  handle_plan: "1. Decompile (jadx + Ghidra) 2. Analyze structure 3. Extract APIs 4. Dynamic test on Cuttlefish"
  status: "processing"
```

### Store Decompiled Source
After decompilation, register the output:
```
register_file
  file_path: "/tmp/decompiled/"
  file_type: "directory"
  summary: "Decompiled source of <package_name> - jadx with deobfuscation"
  status: "analyzed"

# Log the decompilation step
append_file_log
  file_id: "<apk_file_id>"
  log: "Decompiled with jadx --deobf to /tmp/decompiled/. Found X classes, Y activities."
```

### Store Every Finding
As you discover API endpoints, keys, crypto functions, vulnerabilities - record them immediately:
```
# After finding API endpoints
append_file_findings
  file_id: "<apk_file_id>"
  findings: "API Endpoints:\n- POST /api/v1/auth/login (AuthApiService.java:45)\n- GET /api/v1/users/{id} (UserApiService.java:23)\n- Base URL: https://api.target.com\n\nHardcoded Secrets:\n- API_KEY = 'abc123...' (BuildConfig.java:12)\n- AES key = '...' (CryptoHelper.java:78)"

# After Ghidra analysis
append_file_findings
  file_id: "<apk_file_id>"
  findings: "Native lib analysis (libnative.so):\n- JNI_OnLoad at 0x1234 - registers 5 native methods\n- decrypt_payload at 0x5678 - AES-256-CBC, key derived from device ID\n- anti_debug_check at 0x9abc - checks TracerPid, calls ptrace"

# After dynamic analysis
append_file_findings
  file_id: "<apk_file_id>"
  findings: "Dynamic analysis (Cuttlefish + Frida):\n- Login sends credentials to POST /api/v1/auth/login over TLS 1.3\n- Token stored in SharedPreferences (NOT encrypted)\n- Certificate pinning present but bypassed with Frida\n- AES key confirmed via hook: 'k3y_fr0m_d3v1c3'"
```

### Store Persistent Memories for Cross-Session Recall
Use the persistent memory system to remember key findings that should survive conversation resets:
```
# Store the overall analysis summary
store_memory
  key: "re_<package_name>_summary"
  value: "Package: com.target.app v2.1.0\nArchitecture: MVVM + Retrofit\nServer: https://api.target.com\nAuth: Bearer token via /api/v1/auth/login\nCrypto: AES-256-CBC in libnative.so, key from device ID\nAnti-debug: ptrace check in JNI_OnLoad\nVulnerabilities: token stored unencrypted, cert pinning bypassable"

# Store discovered endpoints
store_memory
  key: "re_<package_name>_endpoints"
  value: "POST /api/v1/auth/login - email+password→token\nGET /api/v1/users/{id} - get user profile\nPUT /api/v1/devices - register device\nGET /api/v1/commands - poll for commands (C2-like)\nPOST /api/v1/telemetry - send device data"

# Store crypto findings
store_memory
  key: "re_<package_name>_crypto"
  value: "Algorithm: AES-256-CBC\nKey derivation: SHA256(device_id + hardcoded_salt)\nSalt: 'x7k9m2p4' (CryptoHelper.java:15)\nIV: first 16 bytes of ciphertext\nUsed for: command payload encryption, telemetry data"

# Store the RE methodology used (for future similar APKs)
store_memory
  key: "re_methodology_<app_category>"
  value: "Effective approach for <category> apps:\n1. jadx --deobf first pass\n2. Ghidra for native .so libs\n3. Focus on: auth flow, command channel, crypto\n4. Frida hooks on: encrypt/decrypt, TrustManager, API calls\n5. Key finding locations: BuildConfig, CryptoHelper, ApiService"
```

### Retrieve Previous Analysis
When returning to a previously analyzed APK:
```
# Recall what we found before
retrieve_memory key="re_<package_name>_summary"
retrieve_memory key="re_<package_name>_endpoints"
retrieve_memory key="re_<package_name>_crypto"

# List all RE-related memories
list_memories prefix="re_"

# Get the file manager entry with all findings and logs
list_files type="apk"
get_file file_id="<id>"
```

### Update Progress Throughout
```
# Update file status as work progresses
update_file
  file_id: "<apk_file_id>"
  status: "in_progress"
  progress: "Phase 3/6: Structure analysis complete. Found 12 API endpoints, 3 crypto functions. Starting call flow tracing."

# When done
update_file
  file_id: "<apk_file_id>"
  status: "completed"
  progress: "Full RE complete. 18 endpoints documented, encryption scheme broken, C2 protocol mapped."
```

### Artifact Checklist
For every APK analysis, ensure these artifacts are stored:
- [ ] Original APK registered in File Manager
- [ ] Decompiled source path logged
- [ ] AndroidManifest.xml key findings
- [ ] All discovered API endpoints (with methods, paths, auth)
- [ ] All discovered hardcoded secrets/keys
- [ ] Crypto analysis (algorithms, keys, IVs, key derivation)
- [ ] Native library analysis (if .so files present)
- [ ] Dynamic analysis results (Frida hooks, traffic capture)
- [ ] Vulnerabilities recorded via record_vulnerability tool
- [ ] Summary stored in persistent memory for cross-session access

## Key Patterns to Search

| Pattern | What It Reveals |
|---------|----------------|
| `http://` / `https://` | API endpoints, C2 servers |
| `AES`, `DES`, `RSA`, `encrypt`, `decrypt` | Crypto operations |
| `password`, `token`, `secret`, `key` | Credential handling |
| `SharedPreferences`, `SQLiteDatabase` | Local data storage |
| `TrustManager`, `SSLSocketFactory` | Certificate pinning |
| `Runtime.exec`, `ProcessBuilder` | Command execution |
| `DexClassLoader`, `PathClassLoader` | Dynamic code loading (suspicious) |
| `Base64.encode`, `Base64.decode` | Data encoding (often wraps crypto) |
| `PackageManager`, `getInstallerPackageName` | Anti-tampering checks |
| `Debug.isDebuggerConnected`, `TracerPid` | Anti-debugging |
| `TelephonyManager`, `getDeviceId` | Device fingerprinting |
| `ContentResolver`, `content://` | Content provider access |
