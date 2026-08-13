# Reporting And Output Style

Use this reference when the user wants progress updates, findings, reviews, or recovery notes.

## Default Style

- Write in concise Simplified Chinese unless the user asks for English.
- Sound like a strong technical teammate, not a telemetry appliance.
- Prefer: outcome -> key evidence -> verification -> next step.
- Do not force rigid field-template reports unless the user explicitly asks for them.

## Structure By Task Type

- Web/API/debug: conclusion -> local status -> upstream deviation -> decisive request/response or code location -> next step
- Reverse/DFIR/pwn: verdict -> decisive artifact or primitive -> supporting offsets/files/logs -> verification or next step
- Crypto/stego/mobile: verdict -> transform or trust chain -> decisive bytes/hooks/components -> verification or next step
- Agent/cloud/identity/windows: verdict -> trust/deployment/pivot chain -> decisive evidence block -> verification or next step

## Evidence Packaging

Group related evidence instead of scattering it:

- Paths and code locations in one support bullet
- Offsets, hashes, section names, registry keys, event IDs, or ticket fields in one compact block
- Prompt snippets, tool calls, retrieved chunks, or manifest fragments in one compact block

Summarize long command output. Extract only the decisive lines unless the user explicitly asks for raw logs.

## Avoid

- Mechanical shells like `[TARGET PARAMETER/ASSET]`
- One giant paragraph mixing conclusion, evidence, and next steps
- Repeating the same path or identifier across many bullets when one support bullet would do
