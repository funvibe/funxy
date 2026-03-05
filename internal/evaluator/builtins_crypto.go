package evaluator

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
)

// CryptoBuiltins returns built-in functions for lib/crypto virtual package
func CryptoBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Base64
		"base64Encode": {Fn: builtinBase64Encode, Name: "base64Encode"},
		"base64Decode": {Fn: builtinBase64Decode, Name: "base64Decode"},

		// Hex
		"hexEncode": {Fn: builtinHexEncode, Name: "hexEncode"},
		"hexDecode": {Fn: builtinHexDecode, Name: "hexDecode"},

		// Hashes
		"md5":    {Fn: builtinMd5, Name: "md5"},
		"sha1":   {Fn: builtinSha1, Name: "sha1"},
		"sha256": {Fn: builtinSha256, Name: "sha256"},
		"sha512": {Fn: builtinSha512, Name: "sha512"},

		// HMAC
		"hmacSha256": {Fn: builtinHmacSha256, Name: "hmacSha256"},
		"hmacSha512": {Fn: builtinHmacSha512, Name: "hmacSha512"},

		// Cryptographically secure random
		"cryptoRandomBytes": {Fn: builtinCryptoRandomBytes, Name: "cryptoRandomBytes"},
		"cryptoRandomHex":   {Fn: builtinCryptoRandomHex, Name: "cryptoRandomHex"},
	}
}

// base64Encode: (String) -> String
func builtinBase64Encode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("base64Encode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("base64Encode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	encoded := base64.StdEncoding.EncodeToString([]byte(input))
	return stringToList(encoded)
}

// base64Decode: (String) -> String
func builtinBase64Decode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("base64Decode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("base64Decode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return stringToList("") // Return empty string on decode error
	}
	return stringToList(string(decoded))
}

// hexEncode: (String) -> String
func builtinHexEncode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("hexEncode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("hexEncode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	encoded := hex.EncodeToString([]byte(input))
	return stringToList(encoded)
}

// hexDecode: (String) -> String
func builtinHexDecode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("hexDecode expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("hexDecode expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	decoded, err := hex.DecodeString(input)
	if err != nil {
		return stringToList("") // Return empty string on decode error
	}
	return stringToList(string(decoded))
}

// md5: (String) -> String (hex)
func builtinMd5(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("md5 expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("md5 expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	hash := md5.Sum([]byte(input))
	return stringToList(hex.EncodeToString(hash[:]))
}

// sha1: (String) -> String (hex)
func builtinSha1(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sha1 expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("sha1 expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	hash := sha1.Sum([]byte(input))
	return stringToList(hex.EncodeToString(hash[:]))
}

// sha256: (String) -> String (hex)
func builtinSha256(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sha256 expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("sha256 expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	hash := sha256.Sum256([]byte(input))
	return stringToList(hex.EncodeToString(hash[:]))
}

// sha512: (String) -> String (hex)
func builtinSha512(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sha512 expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("sha512 expects a string, got %s", args[0].Type())
	}

	input := listToString(str)
	hash := sha512.Sum512([]byte(input))
	return stringToList(hex.EncodeToString(hash[:]))
}

// hmacSha256: (String, String) -> String (hex)
func builtinHmacSha256(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("hmacSha256 expects 2 arguments, got %d", len(args))
	}

	keyList, ok := args[0].(*List)
	if !ok {
		return newError("hmacSha256 expects a string key, got %s", args[0].Type())
	}

	msgList, ok := args[1].(*List)
	if !ok {
		return newError("hmacSha256 expects a string message, got %s", args[1].Type())
	}

	key := listToString(keyList)
	msg := listToString(msgList)

	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(msg))
	return stringToList(hex.EncodeToString(h.Sum(nil)))
}

// hmacSha512: (String, String) -> String (hex)
func builtinHmacSha512(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("hmacSha512 expects 2 arguments, got %d", len(args))
	}

	keyList, ok := args[0].(*List)
	if !ok {
		return newError("hmacSha512 expects a string key, got %s", args[0].Type())
	}

	msgList, ok := args[1].(*List)
	if !ok {
		return newError("hmacSha512 expects a string message, got %s", args[1].Type())
	}

	key := listToString(keyList)
	msg := listToString(msgList)

	h := hmac.New(sha512.New, []byte(key))
	h.Write([]byte(msg))
	return stringToList(hex.EncodeToString(h.Sum(nil)))
}

// cryptoRandomBytes: (Int) -> List<Int>
// Returns n cryptographically secure random bytes
func builtinCryptoRandomBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cryptoRandomBytes expects 1 argument, got %d", len(args))
	}

	nInt, ok := args[0].(*Integer)
	if !ok {
		return newError("cryptoRandomBytes expects an integer, got %s", args[0].Type())
	}

	n := int(nInt.Value)
	if n < 0 {
		return newError("cryptoRandomBytes: n cannot be negative")
	}

	if n == 0 {
		return newList([]Object{})
	}

	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		return newError("cryptoRandomBytes: failed to generate random bytes: %s", err.Error())
	}

	elements := make([]Object, n)
	for i, b := range bytes {
		elements[i] = &Integer{Value: int64(b)}
	}

	return newList(elements)
}

// cryptoRandomHex: (Int) -> String
// Returns n cryptographically secure random bytes as hex string
func builtinCryptoRandomHex(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cryptoRandomHex expects 1 argument, got %d", len(args))
	}

	nInt, ok := args[0].(*Integer)
	if !ok {
		return newError("cryptoRandomHex expects an integer, got %s", args[0].Type())
	}

	n := int(nInt.Value)
	if n < 0 {
		return newError("cryptoRandomHex: n cannot be negative")
	}

	if n == 0 {
		return stringToList("")
	}

	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		return newError("cryptoRandomHex: failed to generate random bytes: %s", err.Error())
	}

	return stringToList(hex.EncodeToString(bytes))
}

// SetCryptoBuiltinTypes sets type info for crypto builtins
