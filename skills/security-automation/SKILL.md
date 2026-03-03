---
name: security-automation
description: Professional skills and methodology for security automation
version: 1.0.0
---

# Security Automation

## Overview

Security automation is an important means to improve the efficiency of security operations. This skill provides methods, tools, and best practices for security automation.

## Automation Scenarios

### 1. Vulnerability Scanning

**Automated scanning:**
- Scheduled scanning
- CI/CD integration
- Result analysis
- Report generation

### 2. Security Testing

**Automated testing:**
- Unit testing
- Integration testing
- Security testing
- Regression testing

### 3. Incident Response

**Automated response:**
- Incident detection
- Automatic containment
- Notification and alerting
- Evidence collection

### 4. Compliance Checks

**Automated compliance:**
- Configuration checks
- Policy validation
- Report generation
- Remediation recommendations

## Tools and Frameworks

### Vulnerability Scanning Automation

**Using Nessus API:**
```python
import requests

# Create scan
def create_scan(target, scan_name):
    url = "https://nessus:8834/scans"
    headers = {"X-ApiKeys": "access_key:secret_key"}
    data = {
        "uuid": "template-uuid",
        "settings": {
            "name": scan_name,
            "text_targets": target
        }
    }
    response = requests.post(url, json=data, headers=headers)
    return response.json()

# Launch scan
def launch_scan(scan_id):
    url = f"https://nessus:8834/scans/{scan_id}/launch"
    headers = {"X-ApiKeys": "access_key:secret_key"}
    response = requests.post(url, headers=headers)
    return response.json()
```

**Using OpenVAS API:**
```python
from gvm.connections import UnixSocketConnection
from gvm.protocols.gmp import Gmp

# Connect to OpenVAS
connection = UnixSocketConnection()
gmp = Gmp(connection)
gmp.authenticate('username', 'password')

# Create scan task
target = gmp.create_target(name='target', hosts=['192.168.1.0/24'])
config = gmp.get_configs()[0]
scanner = gmp.get_scanners()[0]

task = gmp.create_task(
    name='scan_task',
    config_id=config['id'],
    target_id=target['id'],
    scanner_id=scanner['id']
)

# Launch scan
gmp.start_task(task['id'])
```

### CI/CD Integration

**Jenkins Pipeline:**
```groovy
pipeline {
    agent any
    stages {
        stage('Security Scan') {
            steps {
                sh 'npm audit'
                sh 'snyk test'
                sh 'sonar-scanner'
            }
        }
        stage('Vulnerability Scan') {
            steps {
                sh 'nmap --script vuln target'
            }
        }
    }
    post {
        always {
            publishHTML([
                reportDir: 'reports',
                reportFiles: 'report.html',
                reportName: 'Security Report'
            ])
        }
    }
}
```

**GitHub Actions:**
```yaml
name: Security Scan

on: [push, pull_request]

jobs:
  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run Snyk
        uses: snyk/actions/node@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
      - name: Run SonarQube
        uses: sonarsource/sonarqube-scan-action@master
        env:
          SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
```

### Security Testing Automation

**Using OWASP ZAP:**
```python
from zapv2 import ZAPv2

# Start ZAP
zap = ZAPv2(proxies={'http': 'http://127.0.0.1:8080'})

# Start scanning
zap.urlopen('http://target.com')
zap.spider.scan('http://target.com')
while int(zap.spider.status()) < 100:
    time.sleep(1)

# Active scan
zap.ascan.scan('http://target.com')
while int(zap.ascan.status()) < 100:
    time.sleep(1)

# Get results
alerts = zap.core.alerts()
```

**Using Burp Suite:**
```python
from burp import IBurpExtender, IScannerCheck

class BurpExtender(IBurpExtender, IScannerCheck):
    def registerExtenderCallbacks(self, callbacks):
        self._callbacks = callbacks
        self._helpers = callbacks.getHelpers()
        callbacks.setExtensionName("Security Automation")
        callbacks.registerScannerCheck(self)

    def doPassiveScan(self, baseRequestResponse):
        # Passive scan logic
        return None

    def doActiveScan(self, baseRequestResponse, insertionPoint):
        # Active scan logic
        return None
```

### Incident Response Automation

**Using Splunk:**
```python
import splunklib.client as client

# Connect to Splunk
service = client.connect(
    host='splunk.example.com',
    port=8089,
    username='admin',
    password='password'
)

# Search security events
search_query = 'index=security event_type="malware"'
kwargs = {"earliest_time": "-1h", "latest_time": "now"}
search = service.jobs.create(search_query, **kwargs)

# Process results
for result in search:
    if result['severity'] == 'high':
        # Automated response
        send_alert(result)
        isolate_system(result['host'])
```

**Using ELK Stack:**
```python
from elasticsearch import Elasticsearch

# Connect to Elasticsearch
es = Elasticsearch(['localhost:9200'])

# Search security events
query = {
    "query": {
        "match": {
            "event_type": "intrusion"
        }
    }
}

results = es.search(index="security", body=query)

# Automated response
for hit in results['hits']['hits']:
    if hit['_source']['severity'] == 'critical':
        # Automatic containment
        block_ip(hit['_source']['src_ip'])
        send_alert(hit['_source'])
```

## Automation Scripts

### Vulnerability Scanning Script

```python
#!/usr/bin/env python3
import subprocess
import json
import smtplib
from email.mime.text import MIMEText

def run_nmap_scan(target):
    """Run Nmap scan"""
    result = subprocess.run(
        ['nmap', '--script', 'vuln', '-oJ', '-', target],
        capture_output=True,
        text=True
    )
    return json.loads(result.stdout)

def analyze_results(results):
    """Analyze scan results"""
    vulnerabilities = []
    for host in results.get('hosts', []):
        for port in host.get('ports', []):
            for script in port.get('scripts', []):
                if script.get('id') == 'vuln':
                    vulnerabilities.append({
                        'host': host['address'],
                        'port': port['portid'],
                        'vuln': script.get('output', '')
                    })
    return vulnerabilities

def send_report(vulnerabilities):
    """Send report"""
    if vulnerabilities:
        msg = MIMEText(f"Found {len(vulnerabilities)} vulnerabilities")
        msg['Subject'] = 'Vulnerability Scan Report'
        msg['From'] = 'security@example.com'
        msg['To'] = 'admin@example.com'

        server = smtplib.SMTP('smtp.example.com')
        server.send_message(msg)
        server.quit()

if __name__ == '__main__':
    target = '192.168.1.0/24'
    results = run_nmap_scan(target)
    vulnerabilities = analyze_results(results)
    send_report(vulnerabilities)
```

### Configuration Check Script

```python
#!/usr/bin/env python3
import boto3
import json

def check_s3_buckets():
    """Check S3 bucket security configuration"""
    s3 = boto3.client('s3')
    buckets = s3.list_buckets()

    issues = []
    for bucket in buckets['Buckets']:
        # Check public access
        try:
            acl = s3.get_bucket_acl(Bucket=bucket['Name'])
            for grant in acl.get('Grants', []):
                if grant.get('Grantee', {}).get('URI') == 'http://acs.amazonaws.com/groups/global/AllUsers':
                    issues.append({
                        'bucket': bucket['Name'],
                        'issue': 'Public access enabled'
                    })
        except:
            pass

        # Check encryption
        try:
            encryption = s3.get_bucket_encryption(Bucket=bucket['Name'])
        except:
            issues.append({
                'bucket': bucket['Name'],
                'issue': 'Encryption not enabled'
            })

    return issues

if __name__ == '__main__':
    issues = check_s3_buckets()
    print(json.dumps(issues, indent=2))
```

## Best Practices

### 1. Automation Strategy

- Identify automatable scenarios
- Develop automation plans
- Implement incrementally
- Continuously improve

### 2. Tool Selection

- Evaluate tool functionality
- Consider integration capabilities
- Consider cost
- Test and validate

### 3. Process Design

- Define process steps clearly
- Define trigger conditions
- Set exception handling
- Record operation logs

### 4. Monitoring and Maintenance

- Monitor automated tasks
- Regularly check results
- Update rules and scripts
- Optimize performance

## Notes

- Ensure automation accuracy
- Set appropriate permissions
- Protect automation credentials
- Regularly review automation rules
