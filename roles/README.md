# Role Configuration Guide

This directory contains all role configuration files. Each role defines the AI's behavior pattern, available tools, and skills.

## Creating a New Role

To create a new role, create a YAML file in the `roles/` directory using the following format:

**Option 1: Explicitly specify the tool list (recommended)**
```yaml
name: Role Name
description: Role description
user_prompt: User prompt (prepended to user messages to guide AI behavior)
icon: "Icon (optional)"
tools:
    # Add the tools you need...
    # ⚠️ Important: it is recommended to include the following 5 built-in MCP tools:
    - record_vulnerability
    - list_knowledge_risk_types
    - search_knowledge_base
    - list_skills
    - read_skill
enabled: true
```

**Option 2: Leave the tools field unset (use all enabled tools)**
```yaml
name: Role Name
description: Role description
user_prompt: User prompt (prepended to user messages to guide AI behavior)
icon: "Icon (optional)"
# Leaving the tools field unset will use all tools enabled in MCP management by default
enabled: true
```

## ⚠️ Important: Built-in MCP Tools

**If the `tools` field is set, make sure to include the following 5 built-in MCP tools in the list:**

1. **`record_vulnerability`** - Vulnerability management tool for recording discovered vulnerabilities
2. **`list_knowledge_risk_types`** - Knowledge base tool for listing available risk types
3. **`search_knowledge_base`** - Knowledge base tool for searching knowledge base content
4. **`list_skills`** - Skills tool for listing available skills
5. **`read_skill`** - Skills tool for reading skill details

These built-in tools are core system features. It is recommended that all roles include them to ensure:
- Ability to record and manage discovered vulnerabilities
- Ability to access the knowledge base for security testing knowledge
- Ability to view and use available security testing skills

**Note**: If the `tools` field is not set, the system defaults to using all tools enabled in MCP management (including these 5 built-in tools). However, for explicit control over the tools available to a role, it is recommended to set the `tools` field explicitly.

## Role Configuration Fields

- **name**: Role name (required)
- **description**: Role description (required)
- **user_prompt**: User prompt - prepended to user messages to guide the AI to adopt specific testing methodologies and focus areas (optional)
- **icon**: Role icon, supports Unicode emoji (optional)
- **tools**: Tool list - specifies the tools available to the role (optional)
  - **If `tools` is not set**: defaults to all tools enabled in MCP management
  - **If `tools` is set**: only the tools specified in the list are used (recommended to include at least the 5 built-in tools)
- **skills**: Skill list - specifies the skills associated with the role (optional)
- **enabled**: Whether to enable the role (required, true/false)

## Examples

Refer to the other role files in this directory, such as `penetration-testing.yaml`, `web-app-scanning.yaml`, etc.
