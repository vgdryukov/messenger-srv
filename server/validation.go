package server

import (
	"regexp"
	"unicode/utf8"
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateUsername(username string) bool {
	if utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 16 {
		return false
	}
	// Только буквы, цифры, подчеркивания и дефисы
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", username)
	return matched
}

func (v *Validator) ValidatePassword(password string) bool {
	if utf8.RuneCountInString(password) < 6 || utf8.RuneCountInString(password) > 32 {
		return false
	}
	return true
}

func (v *Validator) ValidateEmail(email string) bool {
	if email == "" {
		return true // email опциональный
	}
	// Простая валидация email
	matched, _ := regexp.MatchString(`^[^@\s]+@[^@\s]+\.[^@\s]+$`, email)
	return matched
}

func (v *Validator) ValidateMessage(content string) bool {
	if utf8.RuneCountInString(content) == 0 || utf8.RuneCountInString(content) > 1000 {
		return false
	}
	return true
}

func (v *Validator) ValidateGroupName(name string) bool {
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 32 {
		return false
	}
	return true
}
