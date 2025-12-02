package crypto

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// Ошибки хеширования
var (
	ErrEmptyPassword     = errors.New("password cannot be empty")
	ErrPasswordMismatch  = errors.New("password does not match hash")
	ErrInvalidHash       = errors.New("invalid password hash format")
	ErrPasswordTooLong   = errors.New("password exceeds maximum length of 72 bytes")
)

// DefaultCost - стоимость хеширования по умолчанию (рекомендуемое значение)
// Более высокое значение = больше времени на хеширование = более безопасно
const DefaultCost = 12

// MaxPasswordLength - максимальная длина пароля для bcrypt (72 байта)
const MaxPasswordLength = 72

// HashPassword хеширует пароль с использованием bcrypt
// Автоматически генерирует криптографически стойкий salt
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	// bcrypt ограничен 72 байтами
	if len(password) > MaxPasswordLength {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// HashPasswordWithCost хеширует пароль с указанной стоимостью
// cost должен быть от bcrypt.MinCost (4) до bcrypt.MaxCost (31)
func HashPasswordWithCost(password string, cost int) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	if len(password) > MaxPasswordLength {
		return "", ErrPasswordTooLong
	}

	// Валидация cost
	if cost < bcrypt.MinCost {
		cost = bcrypt.MinCost
	}
	if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// VerifyPassword проверяет соответствие пароля хешу
// Использует constant-time comparison для защиты от timing attacks
func VerifyPassword(password, hash string) error {
	if password == "" {
		return ErrEmptyPassword
	}

	if hash == "" {
		return ErrInvalidHash
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrPasswordMismatch
		}
		// Невалидный формат хеша или другая ошибка
		return ErrInvalidHash
	}

	return nil
}

// CheckPasswordMatch проверяет соответствие пароля хешу и возвращает bool
// Удобная обёртка для использования в условиях
func CheckPasswordMatch(password, hash string) bool {
	return VerifyPassword(password, hash) == nil
}

// GetHashCost извлекает cost из существующего хеша
// Полезно для определения необходимости перехеширования при увеличении cost
func GetHashCost(hash string) (int, error) {
	if hash == "" {
		return 0, ErrInvalidHash
	}

	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return 0, ErrInvalidHash
	}

	return cost, nil
}

// NeedsRehash проверяет, нужно ли перехешировать пароль
// Возвращает true если текущий cost хеша меньше желаемого
func NeedsRehash(hash string, desiredCost int) bool {
	currentCost, err := GetHashCost(hash)
	if err != nil {
		return true // При ошибке лучше перехешировать
	}
	return currentCost < desiredCost
}
