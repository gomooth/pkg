package validator

import (
	githubvalidator "github.com/go-playground/validator/v10"
	"github.com/gomooth/xerror"
)

// StructValidator 基于 go-playground/validator 的结构体验证器
type StructValidator struct {
	validate *githubvalidator.Validate
	data     any
}

// NewStructValidator 创建结构体验证器
func NewStructValidator(data any) *StructValidator {
	return &StructValidator{
		validate: githubvalidator.New(),
		data:     data,
	}
}

// Validate 执行验证，返回验证错误
func (sv *StructValidator) Validate() error {
	if sv.data == nil {
		return xerror.New("validator: data is nil")
	}
	if err := sv.validate.Struct(sv.data); err != nil {
		return xerror.Wrap(err, "validator: validation failed")
	}
	return nil
}

// Engine 返回底层验证引擎，用于自定义配置
func (sv *StructValidator) Engine() *githubvalidator.Validate {
	return sv.validate
}
