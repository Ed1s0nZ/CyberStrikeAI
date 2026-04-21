# Skills System Guide

## Overview

The Skills system lets you attach specialized knowledge and skill documents to roles. When a role executes a task, the system adds the skill names to the system prompt as hints. AI agents can then use the `read_skill` tool to retrieve the detailed content of a skill on demand.

## Skills Structure

Each skill is a directory containing a `SKILL.md` file:

```
skills/
├── sql-injection-testing/
│   └── SKILL.md
├── xss-testing/
│   └── SKILL.md
└── ...
```

## SKILL.md Format

`SKILL.md` files support optional YAML front matter:

```markdown
---
name: skill-name
description: Brief description of the skill
version: 1.0.0
---

# Skill Title

Detailed skill content, which may include:
- Testing methods
- Tool usage
- Best practices
- Example code
- etc.
```

If front matter is not used, the entire file content is treated as the skill content.

## Configuring Skills in a Role

Add a `skills` field to the role configuration file:

```yaml
name: Penetration Testing
description: Professional penetration testing expert
user_prompt: You are a professional cybersecurity penetration testing expert...
tools:
  - nmap
  - sqlmap
  - burpsuite
skills:
  - sql-injection-testing
  - xss-testing
enabled: true
```

The `skills` field is a string array where each string is the name of a skill directory.

## How It Works

1. **Loading phase**: At startup, the system scans all skill directories under `skills_dir`.
2. **Execution phase**: When a task is executed with a role:
   - The system adds the skill names configured for that role to the system prompt as recommendations.
   - **Note**: Skill content is **not** automatically injected into the system prompt.
   - AI agents must proactively call the `read_skill` tool to retrieve skill content when needed.
3. **On-demand access**: AI can access skills via the following tools:
   - `list_skills`: Get the list of all available skills.
   - `read_skill`: Read the detailed content of a specified skill.

   This allows AI to autonomously retrieve relevant skills during task execution based on actual need. Even if a role has no skills configured, AI can still access any available skill through these tools on demand.

## Example Skills

### sql-injection-testing

Contains professional methods for SQL injection testing, tool usage, bypass techniques, and more.

### xss-testing

Contains various XSS testing types, payloads, bypass techniques, and more.

## Creating a Custom Skill

1. Create a new directory under `skills/`, e.g., `my-skill`.
2. Create a `SKILL.md` file in that directory.
3. Write the skill content.
4. Add the skill name to the role configuration.

```bash
mkdir -p skills/my-skill
cat > skills/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: My custom skill
---

# My Custom Skill

Skill content goes here...
EOF
```

## Notes

- **Important**: Skill content is NOT automatically injected into the system prompt - only the skill name is added as a hint.
- AI agents must proactively use the `read_skill` tool to retrieve skill content. This saves tokens and improves flexibility.
- Skill content should be clear and structured for AI to understand.
- Code examples and command examples can be included.
- Each skill should focus on a specific domain or technique.
- It is recommended to provide a clear `description` in the YAML front matter to help AI decide whether to read the skill.

## Configuration

Configure the skills directory in `config.yaml`:

```yaml
skills_dir: skills  # Relative to the config file location
```

If not configured, the `skills` directory is used by default.
