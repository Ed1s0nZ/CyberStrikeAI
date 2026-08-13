# Prompt Injection And Tool Boundary Checklist

## Map These Layers Explicitly

- System and developer instructions
- User request
- Retrieved chunks or documents
- Memory or summaries
- Planner draft or chain-of-thought proxy artifacts
- Executor or tool adapter
- Final tool invocation and side effect

## Minimal Proof Chain

Use the smallest chain that proves the bug:

1. Untrusted content enters context
2. Model-visible instruction boundary changes
3. Planner or executor behavior drifts
4. Tool call or secret access changes
5. Side effect becomes observable

## Evidence To Keep

- One compact block for the malicious chunk or prompt
- One compact block for planner drift or intermediate rewrite
- One compact block for final tool args and side effect

## Common Pitfalls

- Treating a malicious string as proof without a side effect
- Mixing several injection variants before one minimal chain is proven
- Forgetting to separate retrieval contamination from executor normalization bugs
