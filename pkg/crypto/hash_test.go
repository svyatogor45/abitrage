package crypto

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestHashPassword проверяет базовое хеширование пароля
func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "password123"},
		{"complex password", "P@ssw0rd!#$%^&*()"},
		{"unicode password", "пароль123"},
		{"long password", strings.Repeat("a", 70)}, // близко к лимиту 72
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword failed: %v", err)
			}

			// Проверяем что хеш не пустой
			if hash == "" {
				t.Error("Hash should not be empty")
			}

			// Проверяем что хеш начинается с $2a$ (bcrypt prefix)
			if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
				t.Errorf("Hash should start with bcrypt prefix, got: %s", hash[:10])
			}

			// Проверяем что хеш отличается от пароля
			if hash == tt.password {
				t.Error("Hash should not equal password")
			}
		})
	}
}

// TestHashPasswordEmptyError проверяет ошибку при пустом пароле
func TestHashPasswordEmptyError(t *testing.T) {
	_, err := HashPassword("")
	if err != ErrEmptyPassword {
		t.Errorf("HashPassword empty: got error %v, want %v", err, ErrEmptyPassword)
	}
}

// TestHashPasswordTooLong проверяет ошибку при слишком длинном пароле
func TestHashPasswordTooLong(t *testing.T) {
	longPassword := strings.Repeat("a", 73) // больше 72 байт
	_, err := HashPassword(longPassword)
	if err != ErrPasswordTooLong {
		t.Errorf("HashPassword too long: got error %v, want %v", err, ErrPasswordTooLong)
	}
}

// TestHashPasswordDifferentHashes проверяет что каждый хеш уникален (разный salt)
func TestHashPasswordDifferentHashes(t *testing.T) {
	password := "samepassword"

	hash1, _ := HashPassword(password)
	hash2, _ := HashPassword(password)

	if hash1 == hash2 {
		t.Error("Two hashes of the same password should be different (different salts)")
	}
}

// TestHashPasswordWithCost проверяет хеширование с разной стоимостью
func TestHashPasswordWithCost(t *testing.T) {
	password := "testpassword"

	tests := []struct {
		name         string
		cost         int
		expectedCost int
	}{
		{"min cost", bcrypt.MinCost, bcrypt.MinCost},
		{"default cost", DefaultCost, DefaultCost},
		{"below min - clamped", 0, bcrypt.MinCost},
		// Не тестируем MaxCost (31), так как это занимает слишком много времени
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPasswordWithCost(password, tt.cost)
			if err != nil {
				t.Fatalf("HashPasswordWithCost failed: %v", err)
			}

			actualCost, _ := GetHashCost(hash)
			if actualCost != tt.expectedCost {
				t.Errorf("Got cost %d, want %d", actualCost, tt.expectedCost)
			}
		})
	}
}

// TestVerifyPassword проверяет верификацию пароля
func TestVerifyPassword(t *testing.T) {
	password := "correctpassword"
	hash, _ := HashPassword(password)

	// Правильный пароль
	err := VerifyPassword(password, hash)
	if err != nil {
		t.Errorf("VerifyPassword with correct password: got error %v, want nil", err)
	}

	// Неправильный пароль
	err = VerifyPassword("wrongpassword", hash)
	if err != ErrPasswordMismatch {
		t.Errorf("VerifyPassword with wrong password: got error %v, want %v", err, ErrPasswordMismatch)
	}
}

// TestVerifyPasswordEmptyInputs проверяет обработку пустых входных данных
func TestVerifyPasswordEmptyInputs(t *testing.T) {
	hash, _ := HashPassword("password")

	// Пустой пароль
	err := VerifyPassword("", hash)
	if err != ErrEmptyPassword {
		t.Errorf("VerifyPassword with empty password: got error %v, want %v", err, ErrEmptyPassword)
	}

	// Пустой хеш
	err = VerifyPassword("password", "")
	if err != ErrInvalidHash {
		t.Errorf("VerifyPassword with empty hash: got error %v, want %v", err, ErrInvalidHash)
	}
}

// TestVerifyPasswordInvalidHash проверяет обработку невалидного хеша
func TestVerifyPasswordInvalidHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"random string", "notahash"},
		{"truncated hash", "$2a$12$abc"},
		{"wrong format", "sha256:abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword("password", tt.hash)
			if err != ErrInvalidHash {
				t.Errorf("VerifyPassword with invalid hash: got error %v, want %v", err, ErrInvalidHash)
			}
		})
	}
}

// TestCheckPasswordMatch проверяет bool-обёртку
func TestCheckPasswordMatch(t *testing.T) {
	password := "testpassword"
	hash, _ := HashPassword(password)

	if !CheckPasswordMatch(password, hash) {
		t.Error("CheckPasswordMatch should return true for correct password")
	}

	if CheckPasswordMatch("wrongpassword", hash) {
		t.Error("CheckPasswordMatch should return false for wrong password")
	}

	if CheckPasswordMatch("", hash) {
		t.Error("CheckPasswordMatch should return false for empty password")
	}
}

// TestGetHashCost проверяет извлечение cost из хеша
func TestGetHashCost(t *testing.T) {
	// Тест с известным cost
	hash, _ := HashPasswordWithCost("password", 10)
	cost, err := GetHashCost(hash)
	if err != nil {
		t.Fatalf("GetHashCost failed: %v", err)
	}
	if cost != 10 {
		t.Errorf("GetHashCost: got %d, want 10", cost)
	}

	// Тест с пустым хешем
	_, err = GetHashCost("")
	if err != ErrInvalidHash {
		t.Errorf("GetHashCost empty: got error %v, want %v", err, ErrInvalidHash)
	}

	// Тест с невалидным хешем
	_, err = GetHashCost("invalid")
	if err != ErrInvalidHash {
		t.Errorf("GetHashCost invalid: got error %v, want %v", err, ErrInvalidHash)
	}
}

// TestNeedsRehash проверяет определение необходимости перехеширования
func TestNeedsRehash(t *testing.T) {
	// Хеш с cost=10
	hash, _ := HashPasswordWithCost("password", 10)

	// Не нужно перехешировать если желаемый cost такой же или меньше
	if NeedsRehash(hash, 10) {
		t.Error("NeedsRehash should return false when cost equals desired")
	}
	if NeedsRehash(hash, 8) {
		t.Error("NeedsRehash should return false when cost is higher than desired")
	}

	// Нужно перехешировать если желаемый cost больше
	if !NeedsRehash(hash, 12) {
		t.Error("NeedsRehash should return true when cost is lower than desired")
	}

	// Невалидный хеш - нужно перехешировать
	if !NeedsRehash("invalid", 10) {
		t.Error("NeedsRehash should return true for invalid hash")
	}
}

// TestDefaultCost проверяет что дефолтный cost соответствует ожиданиям
func TestDefaultCost(t *testing.T) {
	if DefaultCost < 10 {
		t.Errorf("DefaultCost %d is too low for production use", DefaultCost)
	}
	if DefaultCost > 14 {
		t.Errorf("DefaultCost %d may cause performance issues", DefaultCost)
	}
}

// TestHashPasswordWithCostEmpty проверяет ошибку при пустом пароле с cost
func TestHashPasswordWithCostEmpty(t *testing.T) {
	_, err := HashPasswordWithCost("", 10)
	if err != ErrEmptyPassword {
		t.Errorf("HashPasswordWithCost empty: got error %v, want %v", err, ErrEmptyPassword)
	}
}

// TestHashPasswordWithCostTooLong проверяет ошибку при длинном пароле с cost
func TestHashPasswordWithCostTooLong(t *testing.T) {
	longPassword := strings.Repeat("a", 73)
	_, err := HashPasswordWithCost(longPassword, 10)
	if err != ErrPasswordTooLong {
		t.Errorf("HashPasswordWithCost too long: got error %v, want %v", err, ErrPasswordTooLong)
	}
}

// BenchmarkHashPassword измеряет производительность хеширования с дефолтным cost
func BenchmarkHashPassword(b *testing.B) {
	password := "benchmarkpassword123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password)
	}
}

// BenchmarkHashPasswordMinCost измеряет производительность с минимальным cost
func BenchmarkHashPasswordMinCost(b *testing.B) {
	password := "benchmarkpassword123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPasswordWithCost(password, bcrypt.MinCost)
	}
}

// BenchmarkVerifyPassword измеряет производительность верификации
func BenchmarkVerifyPassword(b *testing.B) {
	password := "benchmarkpassword123"
	hash, _ := HashPasswordWithCost(password, bcrypt.MinCost) // MinCost для быстрого бенчмарка

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyPassword(password, hash)
	}
}
