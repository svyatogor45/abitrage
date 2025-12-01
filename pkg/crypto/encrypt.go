package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Ошибки шифрования
var (
	ErrInvalidKeyLength    = errors.New("encryption key must be exactly 32 bytes for AES-256")
	ErrInvalidCiphertext   = errors.New("invalid ciphertext")
	ErrCiphertextTooShort  = errors.New("ciphertext too short")
	ErrDecryptionFailed    = errors.New("decryption failed: authentication error")
)

// Encrypt шифрует plaintext с использованием AES-256-GCM
// Возвращает base64-encoded строку
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}

	// Создаем AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Создаем GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Генерируем случайный nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Шифруем данные
	// GCM добавляет аутентификационный тег автоматически
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Кодируем в base64 для безопасного хранения в БД
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt расшифровывает base64-encoded ciphertext с использованием AES-256-GCM
func Decrypt(ciphertextBase64 string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}

	// Декодируем из base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	// Создаем AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Создаем GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Проверяем минимальную длину (nonce + минимум 1 байт + tag)
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrCiphertextTooShort
	}

	// Извлекаем nonce и зашифрованные данные
	nonce, ciphertextData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Расшифровываем и проверяем аутентификацию
	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// GenerateKey генерирует криптографически стойкий случайный ключ (32 байта для AES-256)
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateKeyString генерирует ключ и возвращает его как строку (для .env файла)
func GenerateKeyString() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return string(key), nil
}

// ValidateKey проверяет, что ключ имеет правильную длину
func ValidateKey(key []byte) error {
	if len(key) != 32 {
		return ErrInvalidKeyLength
	}
	return nil
}

// EncryptWithKeyString — вспомогательная функция для шифрования с строковым ключом
func EncryptWithKeyString(plaintext, keyString string) (string, error) {
	return Encrypt(plaintext, []byte(keyString))
}

// DecryptWithKeyString — вспомогательная функция для расшифровки с строковым ключом
func DecryptWithKeyString(ciphertextBase64, keyString string) (string, error) {
	return Decrypt(ciphertextBase64, []byte(keyString))
}
