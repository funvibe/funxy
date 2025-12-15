# Cryptography and Encoding (lib/crypto)

The `lib/crypto` module provides functions for encoding, hashing, and HMAC.

```rust
import "lib/crypto" (*)
```

## Encoding Functions

### Base64

```rust
base64Encode(s: String) -> String
base64Decode(s: String) -> String
```

Encoding and decoding in Base64.

```rust
import "lib/crypto" (base64Encode, base64Decode)

encoded = base64Encode("Hello, World!")
print(encoded)  // SGVsbG8sIFdvcmxkIQ==

decoded = base64Decode(encoded)
print(decoded)  // Hello, World!

// Encoding binary data for transmission
data = "some binary data"
safe = base64Encode(data)
// transmit safe over network
original = base64Decode(safe)
```

### Hex (hexadecimal)

```rust
hexEncode(s: String) -> String
hexDecode(s: String) -> String
```

Encoding to hexadecimal representation and back.

```rust
import "lib/crypto" (hexEncode, hexDecode)

hex = hexEncode("ABC")
print(hex)  // 414243

original = hexDecode(hex)
print(original)  // ABC
```

## Hash Functions

All hash functions return results as hexadecimal strings.

### MD5

```rust
md5(s: String) -> String
```

**Warning:** MD5 is considered cryptographically insecure. Use only for checksums, not for security.

```rust
import "lib/crypto" (md5)

hash = md5("hello")
print(hash)  // 5d41402abc4b2a76b9719d911017c592
print(len(hash))  // 32 (hex chars)
```

### SHA1

```rust
sha1(s: String) -> String
```

**Warning:** SHA1 is considered deprecated for cryptographic purposes.

```rust
import "lib/crypto" (sha1)

hash = sha1("hello")
print(hash)  // aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
print(len(hash))  // 40 (hex chars)
```

### SHA256

```rust
sha256(s: String) -> String
```

Recommended algorithm for most tasks.

```rust
import "lib/crypto" (sha256)

hash = sha256("hello")
print(hash)  // 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
print(len(hash))  // 64 (hex chars)

// Password hashing (simple example)
passwordHash = sha256("mypassword" ++ "salt123")
```

### SHA512

```rust
sha512(s: String) -> String
```

Longer hash for enhanced security.

```rust
import "lib/crypto" (sha512)

hash = sha512("hello")
print(len(hash))  // 128 (hex chars)
```

## HMAC

HMAC (Hash-based Message Authentication Code) — message authentication code using a hash function and secret key.

### hmacSha256

```rust
hmacSha256(key: String, message: String) -> String
```

```rust
import "lib/crypto" (hmacSha256)

signature = hmacSha256("secret-key", "message to sign")
print(signature)  // 64 hex chars

// Signature verification
expectedSig = hmacSha256("secret-key", "message to sign")
if signature == expectedSig {
    print("Valid signature")
}
```

### hmacSha512

```rust
hmacSha512(key: String, message: String) -> String
```

```rust
import "lib/crypto" (hmacSha512)

signature = hmacSha512("secret-key", "message")
print(len(signature))  // 128 hex chars
```

## Practical Examples

### Simple API Request Signature

```rust
import "lib/crypto" (hmacSha256, sha256)
import "lib/time" (timeNow)

fun signRequest(apiKey: String, secretKey: String, body: String) -> String {
    timestamp = show(timeNow())
    payload = timestamp ++ body
    hmacSha256(secretKey, payload)
}

// Usage
apiKey = "my-api-key"
secretKey = "my-secret"
body = "{\"action\": \"buy\"}"

signature = signRequest(apiKey, secretKey, body)
print("X-Signature: ${signature}")
```

### File Checksum

```rust
import "lib/crypto" (sha256)
import "lib/io" (fileRead)

fun fileChecksum(path: String) -> String {
    match fileRead(path) {
        Ok(content) -> sha256(content)
        Fail(_) -> ""
    }
}

checksum = fileChecksum("myfile.txt")
print("SHA256: ${checksum}")
```

### Token Generation

```rust
import "lib/crypto" (sha256, base64Encode)
import "lib/time" (clockNs)

fun generateToken(userId: String) -> String {
    // Simple token based on time and userId
    data = userId ++ show(clockNs())
    hash = sha256(data)
    base64Encode(hash)
}

token = generateToken("user123")
print("Token: ${token}")
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `base64Encode` | `String -> String` | Base64 encoding |
| `base64Decode` | `String -> String` | Base64 decoding |
| `hexEncode` | `String -> String` | Hex encoding |
| `hexDecode` | `String -> String` | Hex decoding |
| `md5` | `String -> String` | MD5 hash (32 hex) |
| `sha1` | `String -> String` | SHA1 hash (40 hex) |
| `sha256` | `String -> String` | SHA256 hash (64 hex) |
| `sha512` | `String -> String` | SHA512 hash (128 hex) |
| `hmacSha256` | `(String, String) -> String` | HMAC-SHA256 |
| `hmacSha512` | `(String, String) -> String` | HMAC-SHA512 |

## Security Recommendations

1. **Do not use MD5 or SHA1** for cryptographic purposes (passwords, signatures).
2. **Use SHA256** as the standard choice for hashing.
3. **Use HMAC** for message authentication, not simple hash.
4. **Store secret keys** in environment variables, not in code.
5. **For passwords** use specialized algorithms (bcrypt, argon2) — they are not included in this module.
