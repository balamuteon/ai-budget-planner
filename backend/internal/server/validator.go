package server

import "github.com/go-playground/validator/v10"

type CustomValidator struct {
	validator *validator.Validate
}

// NewValidator создает валидатор на базе go-playground/validator.
func NewValidator() *CustomValidator {
	v := validator.New()
	return &CustomValidator{validator: v}
}

// Validate запускает проверку структуры по тегам.
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}
