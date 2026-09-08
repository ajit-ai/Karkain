# 14 Security — Checksum & Input Validation

STATUS: WORKING TODAY, NON-CRYPTOGRAPHIC (validated).

## What it demonstrates

- a deterministic 32-bit rolling hash (`h = (h*33 + x) mod 1_000_000_007`)
  over an integer payload — integrity-style checksumming in pure Karkain
- canonical unsigned-decimal validation via single-character range checks
  (`c >= "0" && c <= "9"` on `s[i]`)
- a simple heuristic password "strength" score (length + digit presence)

## HONEST SCOPE

The runtime provides NO cryptographic primitives (no hash/HMAC/cipher/secure
RNG). This example is integrity/validation arithmetic, NOT a security
boundary. Numeric conversion via `int(string)` is also unavailable on the
default kcc engine today, which is exactly why validation is character-based.

## Commands

```
karkain check examples/showcase/14_security/checksum/main.kark
karkain run   examples/showcase/14_security/checksum/main.kark
```

## Expected output (verified)

```
checksum=381823039
checksum_changed=381823072
valid_12345=1
valid_12a45=0
valid_empty=0
password_abcdef=0
password_str0ng!=1
```