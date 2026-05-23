package validator

import (
	"testing"

	v10 "github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"gte=0,lte=150"`
}

func TestStructValidator_Success(t *testing.T) {
	data := testStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	v := NewStructValidator(data)
	err := v.Validate()

	assert.NoError(t, err)
}

func TestStructValidator_MissingRequired(t *testing.T) {
	data := testStruct{
		Name:  "",
		Email: "john@example.com",
		Age:   30,
	}

	v := NewStructValidator(data)
	err := v.Validate()

	assert.Error(t, err)

	// Check if the error contains information about the Name field
	assert.Contains(t, err.Error(), "Name")
}

func TestStructValidator_InvalidEmail(t *testing.T) {
	data := testStruct{
		Name:  "John Doe",
		Email: "invalid-email",
		Age:   30,
	}

	v := NewStructValidator(data)
	err := v.Validate()

	assert.Error(t, err)

	// Check if the error contains information about the Email field
	assert.Contains(t, err.Error(), "Email")
}

func TestStructValidator_InvalidAge(t *testing.T) {
	// Test age too low
	data1 := testStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   -1,
	}

	v1 := NewStructValidator(data1)
	err1 := v1.Validate()

	assert.Error(t, err1)

	// Test age too high
	data2 := testStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   151,
	}

	v2 := NewStructValidator(data2)
	err2 := v2.Validate()

	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "Age")
}

func TestStructValidator_Engine(t *testing.T) {
	data := testStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	v := NewStructValidator(data)
	engine := v.Engine()

	require.NotNil(t, engine)
	assert.IsType(t, &v10.Validate{}, engine)
}

func TestStructValidator_NestedValidation(t *testing.T) {
	type address struct {
		Street string `validate:"required"`
		City   string `validate:"required"`
		Zip    string `validate:"required,len=5"`
	}

	type user struct {
		Name    string  `validate:"required"`
		Email   string  `validate:"required,email"`
		Age     int     `validate:"gte=0,lte=150"`
		Address address `validate:"required"`
	}

	data := user{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
		Address: address{
			Street: "123 Main St",
			City:   "",
			Zip:    "1234", // Invalid length
		},
	}

	v := NewStructValidator(data)
	err := v.Validate()

	assert.Error(t, err)

	// Check if the error contains information about the nested Address field
	assert.Contains(t, err.Error(), "Address")
}

func TestStructValidator_NilData(t *testing.T) {
	var data *testStruct = nil

	v := NewStructValidator(data)
	err := v.Validate()

	assert.Error(t, err)
	// The error message will include "validator:" prefix and the nil value
	assert.Contains(t, err.Error(), "validator:")
}

func TestIValidator_Interface(t *testing.T) {
	data := testStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	var v IValidator = NewStructValidator(data)
	err := v.Validate()

	assert.NoError(t, err)
}
