# Custom Protocol Replay Checklist

## Transcript First

- Establish roles, stream IDs, ports, session resets, handshake boundaries, and successful transcript examples
- Separate transport segmentation from application messages
- Record retransmits or duplicate messages so they do not pollute the protocol model

## Recovery Order

1. Framing and message boundaries
2. Direction and sequence state
3. Integrity fields: checksum, MAC, counter, nonce, or signature
4. Compression, encoding, or crypto boundary
5. Accepted vs rejected transcript deltas

## Evidence To Keep Together

- Message identity: type, offset, length, direction, sequence, or delimiter
- Acceptance state: prior message dependency, negotiated value, checksum, or nonce source
- Replay proof: minimal transcript, harness input, and server response or side effect

## Common Pitfalls

- Trying broad replay before framing and state dependencies are understood
- Mixing messages from separate sessions into one replay model
- Claiming protocol recovery without producing an accepted replay or meaningful state transition
