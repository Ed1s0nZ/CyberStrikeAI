# File Parser Chain Checklist

## First Pass

- Upload request shape, field names, content type, filename, extension, magic bytes, temp storage
- Archive members, conversion outputs, previews, thumbnails, extracted docs, serialized objects
- Final consumers: parser, converter, renderer, importer, deserializer, worker

## Chain To Reconstruct

1. File ingress accepted
2. Temp or staged artifact created
3. Extraction or conversion performed
4. Parser or deserializer invoked
5. Business-logic effect or artifact produced

## Evidence To Keep Together

- Ingress side: request, filename, MIME, temp path, storage key
- Parser side: tool or library invoked, branch condition, derived artifact, parser choice
- Effect side: rendered output, parsed object, backend branch, worker task, or privilege-bearing result

## Common Pitfalls

- Looking only at the original upload and ignoring derived intermediates
- Treating MIME or extension checks as proof of backend parser choice
- Mixing archive, preview, and deserialization stages without preserving each boundary separately
