package struct_flags

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestFlagValidate(t *testing.T) {

	type testFlags struct {
		File string `flag:"out" validate:"required,file=exists"`
	}

	args := testFlags{}

	require.Error(t, ValidateStructFields(args))

	args = testFlags{
		File: "validate_test.go",
	}

	require.NoError(t, ValidateStructFields(args))

	args = testFlags{
		File: "validate_test.go.missing",
	}

	require.Error(t, ValidateStructFields(args))

	type testFlags2 struct {
		File string `flag:"out" validate:"file=absolute"`
	}

	args2 := testFlags2{}

	require.NoError(t, ValidateStructFields(args2))

	args2 = testFlags2{
		File: "validate_test.go",
	}

	require.Error(t, ValidateStructFields(args2))

	abs, _ := filepath.Abs("validate_test.go")

	args2 = testFlags2{
		File: abs,
	}

	require.NoError(t, ValidateStructFields(args2))

}
