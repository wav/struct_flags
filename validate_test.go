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

	require.Error(t, PrepareStructFields(args))

	args = testFlags{
		File: "validate_test.go",
	}

	require.NoError(t, PrepareStructFields(args))

	args = testFlags{
		File: "validate_test.go.missing",
	}

	require.Error(t, PrepareStructFields(args))

	type testFlags2 struct {
		File string `flag:"out" validate:"file=absolute"`
	}

	args2 := testFlags2{}

	require.NoError(t, PrepareStructFields(args2))

	args2 = testFlags2{
		File: "validate_test.go",
	}

	require.Error(t, PrepareStructFields(args2))

	abs, _ := filepath.Abs("validate_test.go")

	args2 = testFlags2{
		File: abs,
	}

	require.NoError(t, PrepareStructFields(args2))

}
