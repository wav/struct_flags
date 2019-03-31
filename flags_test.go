package struct_flags

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/go-playground/validator.v9"
	"io/ioutil"
	"os"
	"testing"
)

func TestFlagSet_UnmarshalFlags(t *testing.T) {

	type Object struct {
		String1 string `flag:"string1" usage:"nested.string1"`
		String2 string `flag:"string2" usage:"nested.string2"`
	}

	type Flags struct {
		String      string            `flag:"string" usage:"string"`
		Int         int               `flag:"int" usage:"int"`
		Bool        bool              `flag:"bool" env:"BOOL" usage:"bool"`
		List        []string          `flag:"list" usage:"list"`
		NestedFlags Object            `flag:"nested" usage:"nested"`
		Map         map[string]string `flag:"map" usage:"map"`
	}

	fs := NewFlagSet(&Flags{})
	flags := Flags{}
	args, err := fs.UnmarshalFlags([]string{}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, args)

	fs = NewFlagSet(&Flags{String: "default"})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, args)
	assert.Equal(t, "default", flags.String)

	fs = NewFlagSet(&Flags{String: "default"})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{"--string=test"}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, args)
	assert.Equal(t, "test", flags.String)

	fs = NewFlagSet(&Flags{Bool: true})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{"--bool=true"}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, args)
	assert.Equal(t, true, flags.Bool)

	fs = NewFlagSet(&Flags{Int: 2})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{"--int=1", "arg1", "arg2"}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{"arg1", "arg2"}, args)
	assert.Equal(t, 1, flags.Int)

	fs = NewFlagSet(&Flags{Int: 2, NestedFlags: Object{
		String1: "default",
	}})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{"--list=a,b,c", "--nested.string2=d", "--map=a=1,b=2", "--map=x=z", "--list=d", "arg1", "arg2"}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{"arg1", "arg2"}, args)
	assert.Equal(t, 2, flags.Int)
	assert.Equal(t, []string{"a", "b", "c", "d"}, flags.List)
	assert.Equal(t, "default", flags.NestedFlags.String1)
	assert.Equal(t, "d", flags.NestedFlags.String2)
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "x": "z"}, flags.Map)

	require.NoError(t, os.Setenv("BOOL", "true"))

	fs = NewFlagSet(&Flags{})
	flags = Flags{}
	args, err = fs.UnmarshalFlags([]string{}, &flags)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, args)
	assert.Equal(t, true, flags.Bool)

}

func TestFlagSetUsage(t *testing.T) {

	type ex1 struct {
		String string `flag:"string" validate:"required"`
	}

	prepare := NewCommand("cmd", "", ex1{}, nil, func(_ context.Context, flags ex1) error {
		return nil
	}).PrepareFlags

	err := prepare(ex1{})
	verr, ok := err.(validator.ValidationErrors)
	require.True(t, ok)
	f, ok := getStructFieldForError(verr[0], ex1{})
	require.True(t, ok)
	println(f.Tag.Get("flag"))
}

func TestNestedCommand(t *testing.T) {

	type cmd struct {
		String string `flag:"string" validate:"required"`
	}

	value := ""

	command := NewCommand("top", "", cmd{}, nil, func(_ context.Context, flags cmd) error {
		value = "top " + flags.String
		return nil
	})

	subCommand := NewCommand("cmd", "", cmd{}, nil, func(_ context.Context, flags cmd) error {
		value = "sub " + flags.String
		return nil
	})

	commands := Commands{command, NewCommandGroup("has_sub", "", subCommand)}

	require.NoError(t, commands.Run(context.TODO(), []string{"<exe>", "top", "--string=1"}))

	require.Equal(t, "top 1", value)

	require.NoError(t, commands.Run(context.TODO(), []string{"<exe>", "has_sub", "cmd", "--string=v"}))

	require.Equal(t, "sub v", value)

}

func TestArgFile(t *testing.T) {

	type cmd struct {
		String          string `flag:"string" validate:"required"`
		StringFromEnv   string `flag:"stringFromEnv" env:"STRING_FROM_ENV" validate:"required"`
		IntFromEnvValue string `flag:"intFromEnvValue" validate:"required"`
	}

	var argFile *ArgFile
	var collectedFlags cmd

	command := NewCommand("cmd", "", cmd{}, nil, func(ctx context.Context, flags cmd) error {
		argFile = getArgFile(ctx)
		collectedFlags = flags
		return nil
	})

	commands := Commands{command}

	argFileFile, err := ioutil.TempFile("", t.Name()+"-argfile.txt")
	require.NoError(t, err)
	argFileFilename := argFileFile.Name()
	defer argFileFile.Close()
	defer os.RemoveAll(argFileFilename)
	argFileData, _ := json.Marshal(ArgFile{
		Command: []string{"cmd"},
		Args:    []string{"--string=test", "--intFromEnvValue=$b"},
		Env: []string{
			"b=1",
			"STRING_FROM_ENV=a${b}",
		},
	})
	_, err = argFileFile.Write(argFileData)
	require.NoError(t, err)

	argFileFile.Close()

	require.NoError(t, commands.Run(context.TODO(), []string{"<exe>", "@" + argFileFilename}))
	require.NotNil(t, argFile)

	require.Equal(t, "test", collectedFlags.String)
	require.Equal(t, "a1", collectedFlags.StringFromEnv)
	require.Equal(t, "1", collectedFlags.IntFromEnvValue)
}
