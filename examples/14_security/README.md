# 14 — Security

Safe, educational security examples built on the implemented
`std.crypto` / `std.encoding` primitives. All data is synthetic and public
test data. No offensive tooling; no fabricated cryptographic APIs.

| Example                       | Status   | Engine | Idea                                   |
|-------------------------------|----------|--------|----------------------------------------|
| `01_digests.kark`             | Runnable | both   | sha256/sha512 vs NIST vectors          |
| `02_encoding_roundtrip.kark`  | Runnable | both   | hex/base64 RFC 4648 round-trips        |
| `03_password_hash.kark`       | Runnable | both   | salt + sha256 demo (educational only)  |
| `04_utf8_text.kark`           | Runnable | both   | UTF-8 encode/decode/validate           |

## Honest scope

- No secure random source in the stdlib; no AES/RSA/ECC; no key exchange.
  `03_password_hash.kark` is deliberately labeled a demonstration, not a
  production KDF.
- Malformed hex/base64 input raises the same source-located runtime error on
  both engines (`runtime error: invalid (hex|base64) string at file:line`).

## Run

```
karkain run examples/14_security/01_digests.kark
```