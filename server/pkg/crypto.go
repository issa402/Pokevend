// ============================================================
// FILE: server/pkg/crypto.go
// TYPE: Shared Utility — AES-256-GCM Encryption
//
// WHAT IS THIS?
// Provides Encrypt() and Decrypt() for securing API keys stored in PostgreSQL.
// Used by handlers/apikey_handler.go to protect eBay/TCGplayer credentials.
//
// WHY ENCRYPT API KEYS BEFORE STORING?
// "Defense in depth" — multiple layers of security.
// Even if an attacker gets database access (via SQL injection, leaked backup, etc.),
// they see AES-encrypted hex strings, NOT real API keys.
// They'd need BOTH the database AND the ENCRYPTION_KEY env var to decrypt.
//
// AES-256-GCM:
//   AES = Advanced Encryption Standard (symmetric — same key encrypts and decrypts)
//   256 = key length in bits (32 bytes) — maximum AES strength
//   GCM = Galois/Counter Mode — an authenticated encryption mode
//         GCM verifies data integrity: if ciphertext is tampered with, decryption fails
//         This prevents attackers from modifying encrypted data without detection
//
// NONCE (Number Used Once):
//   Each encryption uses a random 12-byte nonce (initialization vector).
//   Even if you encrypt the same key twice, the output is different each time.
//   The nonce is prepended to the ciphertext and extracted during decryption.
//   Without nonce: encrypt("key") = always same output → pattern analysis attack.
//
// FAANG SECURITY STANDARD:
//   At Stripe, Twilio, and AWS — credentials stored encrypted with KMS (Key Management Service).
//   Our approach mimics this using AES-256-GCM with an env var key.
//   In production: use AWS KMS or HashiCorp Vault instead of a static key.
//
// GO CONCEPTS:
//   crypto/aes, crypto/cipher, crypto/rand, encoding/hex
// ============================================================
package pkg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext with AES-256-GCM using keyHex (a 64-char hex string = 32 bytes).
// Returns the encrypted value as a hex string: nonce(24 hex chars) + ciphertext(hex)
// This hex string is what gets stored in the api_keys.encrypted_key column.
func Encrypt(plaintext, keyHex string) (string, error) {
	// hex.DecodeString converts "0a1b2c..." → actual bytes
	// keyHex must be exactly 64 hex chars (= 32 bytes = AES-256)
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("invalid key: must be 64 hex chars (32 bytes)")
	}

	// aes.NewCipher creates the AES block cipher with our key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// cipher.NewGCM wraps the block cipher with GCM mode
	// GCM adds authentication — detect if ciphertext was tampered with
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate a cryptographically random nonce
	// gcm.NonceSize() = 12 bytes — this is the GCM standard nonce size
	nonce := make([]byte, gcm.NonceSize())
	// io.ReadFull reads exactly len(nonce) random bytes from crypto/rand
	// crypto/rand = OS random number generator (secure, not predictable)
	// NEVER use math/rand for security — it's pseudorandom and predictable
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// gcm.Seal encrypts and authenticates:
	//   dst = nonce (prepend nonce to the ciphertext)
	//   nonce = nonce to use for encryption
	//   plaintext = data to encrypt
	//   nil = no "additional data" (AAD) — optional extra authenticated context
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	// ciphertext now = nonce + encrypted_data (nonce prepended)

	// Return as hex string for safe storage in PostgreSQL TEXT column
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt() — extracts the nonce, decrypts the ciphertext.
// Returns the original plaintext API key.
func Decrypt(ciphertextHex, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("invalid key")
	}

	// Decode the hex-encoded ciphertext back to raw bytes
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Extract the nonce from the beginning of data
	// data = nonce(12 bytes) + ciphertext
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// gcm.Open decrypts and VERIFIES the authentication tag
	// If the ciphertext was tampered with → error: "cipher: message authentication failed"
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (data may be tampered)")
	}

	return string(plaintext), nil
}

// TODO #1 (Practice): Add key rotation support
// If ENCRYPTION_KEY is ever compromised, you need to re-encrypt all keys.
// Add: func ReEncrypt(ciphertextHex, oldKeyHex, newKeyHex string) (string, error)
// Process: decrypt with old key → encrypt with new key.
// Store ENCRYPTION_KEY_VERSION in .env (v1, v2...) and track which version
// encrypted each api_key row (add encrypted_key_version to the api_keys table).
// This is how AWS KMS handles key rotation.

// TODO #2 (Practice): Add HMAC for short values that don't need decryption
// For webhook secrets (we just need to VERIFY matches, not decrypt):
// Use HMAC-SHA256 instead of AES: func Sign(value, secret string) string
//   h := hmac.New(sha256.New, []byte(secret))
//   h.Write([]byte(value))
//   return hex.EncodeToString(h.Sum(nil))
// HMAC = one-way. You sign webhook payloads with it and compare.
// This is how eBay's webhook signature verification works.
