package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestEncryptDecrypt проверяет базовый цикл шифрования/расшифровки
func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"simple text", "Hello, World!"},
		{"api key example", "abc123def456ghi789"},
		{"unicode text", "Привет мир 你好世界"},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"long text", strings.Repeat("a", 1000)},
		{"json data", `{"api_key": "secret", "api_secret": "very_secret"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Проверяем что результат - валидный base64
			_, err = base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Errorf("Encrypted result is not valid base64: %v", err)
			}

			// Проверяем что зашифрованный текст отличается от оригинала
			if encrypted == tt.plaintext && tt.plaintext != "" {
				t.Error("Encrypted text should not equal plaintext")
			}

			decrypted, err := Decrypt(encrypted, key)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypted text mismatch: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

// TestEncryptDifferentResults проверяет что каждое шифрование даёт разный результат (разный nonce)
func TestEncryptDifferentResults(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := "same text"

	encrypted1, _ := Encrypt(plaintext, key)
	encrypted2, _ := Encrypt(plaintext, key)

	if encrypted1 == encrypted2 {
		t.Error("Two encryptions of the same text should produce different ciphertexts")
	}

	// Оба должны расшифровываться корректно
	decrypted1, _ := Decrypt(encrypted1, key)
	decrypted2, _ := Decrypt(encrypted2, key)

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both ciphertexts should decrypt to the same plaintext")
	}
}

// TestEncryptInvalidKeyLength проверяет ошибку при неправильной длине ключа
func TestEncryptInvalidKeyLength(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr error
	}{
		{"too short - 16 bytes", 16, ErrInvalidKeyLength},
		{"too short - 31 bytes", 31, ErrInvalidKeyLength},
		{"too long - 33 bytes", 33, ErrInvalidKeyLength},
		{"too long - 64 bytes", 64, ErrInvalidKeyLength},
		{"empty key", 0, ErrInvalidKeyLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := Encrypt("test", key)
			if err != tt.wantErr {
				t.Errorf("Encrypt with %d byte key: got error %v, want %v", tt.keyLen, err, tt.wantErr)
			}
		})
	}
}

// TestDecryptInvalidKeyLength проверяет ошибку расшифровки при неправильном ключе
func TestDecryptInvalidKeyLength(t *testing.T) {
	// Сначала создадим валидный шифротекст
	validKey, _ := GenerateKey()
	encrypted, _ := Encrypt("test", validKey)

	tests := []struct {
		name    string
		keyLen  int
		wantErr error
	}{
		{"too short", 16, ErrInvalidKeyLength},
		{"too long", 64, ErrInvalidKeyLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := Decrypt(encrypted, key)
			if err != tt.wantErr {
				t.Errorf("Decrypt with %d byte key: got error %v, want %v", tt.keyLen, err, tt.wantErr)
			}
		})
	}
}

// TestDecryptWrongKey проверяет что расшифровка с неправильным ключом возвращает ошибку
func TestDecryptWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	encrypted, _ := Encrypt("secret data", key1)

	_, err := Decrypt(encrypted, key2)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt with wrong key: got error %v, want %v", err, ErrDecryptionFailed)
	}
}

// TestDecryptInvalidBase64 проверяет обработку невалидного base64
func TestDecryptInvalidBase64(t *testing.T) {
	key, _ := GenerateKey()

	tests := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{"not base64", "not-valid-base64!!!", ErrInvalidCiphertext},
		{"truncated base64", "YWJj", ErrCiphertextTooShort}, // слишком короткий после декодирования
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext, key)
			if err != tt.wantErr {
				t.Errorf("Decrypt(%q): got error %v, want %v", tt.ciphertext, err, tt.wantErr)
			}
		})
	}
}

// TestDecryptTamperedCiphertext проверяет обнаружение изменённого шифротекста
func TestDecryptTamperedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	encrypted, _ := Encrypt("original data", key)

	// Декодируем, меняем байт, кодируем обратно
	decoded, _ := base64.StdEncoding.DecodeString(encrypted)
	if len(decoded) > 20 {
		decoded[20] ^= 0xFF // Инвертируем байт
	}
	tampered := base64.StdEncoding.EncodeToString(decoded)

	_, err := Decrypt(tampered, key)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt tampered ciphertext: got error %v, want %v", err, ErrDecryptionFailed)
	}
}

// TestGenerateKey проверяет генерацию ключей
func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("GenerateKey: got %d bytes, want 32", len(key1))
	}

	// Генерируем второй ключ и проверяем что они разные
	key2, _ := GenerateKey()
	if string(key1) == string(key2) {
		t.Error("Two generated keys should be different")
	}
}

// TestGenerateKeyString проверяет генерацию ключа как строки
func TestGenerateKeyString(t *testing.T) {
	keyStr, err := GenerateKeyString()
	if err != nil {
		t.Fatalf("GenerateKeyString failed: %v", err)
	}

	if len(keyStr) != 32 {
		t.Errorf("GenerateKeyString: got %d bytes, want 32", len(keyStr))
	}
}

// TestValidateKey проверяет валидацию длины ключа
func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr error
	}{
		{"valid 32 bytes", 32, nil},
		{"too short", 16, ErrInvalidKeyLength},
		{"too long", 64, ErrInvalidKeyLength},
		{"empty", 0, ErrInvalidKeyLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			err := ValidateKey(key)
			if err != tt.wantErr {
				t.Errorf("ValidateKey(%d bytes): got error %v, want %v", tt.keyLen, err, tt.wantErr)
			}
		})
	}
}

// TestEncryptWithKeyString проверяет шифрование со строковым ключом
func TestEncryptWithKeyString(t *testing.T) {
	keyString := "12345678901234567890123456789012" // 32 символа

	encrypted, err := EncryptWithKeyString("test data", keyString)
	if err != nil {
		t.Fatalf("EncryptWithKeyString failed: %v", err)
	}

	decrypted, err := DecryptWithKeyString(encrypted, keyString)
	if err != nil {
		t.Fatalf("DecryptWithKeyString failed: %v", err)
	}

	if decrypted != "test data" {
		t.Errorf("Got %q, want %q", decrypted, "test data")
	}
}

// TestEncryptWithKeyStringInvalidLength проверяет ошибку при неправильной длине строкового ключа
func TestEncryptWithKeyStringInvalidLength(t *testing.T) {
	shortKey := "short"
	_, err := EncryptWithKeyString("test", shortKey)
	if err != ErrInvalidKeyLength {
		t.Errorf("EncryptWithKeyString with short key: got error %v, want %v", err, ErrInvalidKeyLength)
	}
}

// BenchmarkEncrypt измеряет производительность шифрования
func BenchmarkEncrypt(b *testing.B) {
	key, _ := GenerateKey()
	plaintext := "This is a typical API key: abc123def456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Encrypt(plaintext, key)
	}
}

// BenchmarkDecrypt измеряет производительность расшифровки
func BenchmarkDecrypt(b *testing.B) {
	key, _ := GenerateKey()
	encrypted, _ := Encrypt("This is a typical API key: abc123def456", key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decrypt(encrypted, key)
	}
}

// BenchmarkEncryptDecryptCycle измеряет полный цикл
func BenchmarkEncryptDecryptCycle(b *testing.B) {
	key, _ := GenerateKey()
	plaintext := "This is a typical API key: abc123def456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, _ := Encrypt(plaintext, key)
		_, _ = Decrypt(encrypted, key)
	}
}
