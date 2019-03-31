package main

import (
	"context"
	"encoding/json"
	"github.com/wav/struct_flags"
	"os"
	"strings"
)

// commands for this package
var commands struct_flags.Commands

// main for this package
func main() {
	if err := commands.Run(context.TODO(), os.Args); err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

// init is used to register commands per file
func init() {
	c := struct_flags.NewCommand("print-args", Flags{}, "print the provided arguments if they validate ok", execute)
	commands = append(commands, c, struct_flags.NewCommandGroup("more", ""))
}

type Object struct {
	String1 string `flag:"string1" usage:"string1 if squashed, otherwise nested.string1"`
	String2 string `flag:"string2" usage:"string2 if squashed, otherwise nested.string2"`
}

type Flags struct {
	String        string            `flag:"string" usage:"string"`
	Filepath      string            `flag:"filepath" usage:"filepath" validate:"required,file=absolute,file=exists"`
	Int           int               `flag:"int" usage:"int"`
	Bool          bool              `flag:"bool" env:"BOOL" usage:"bool"`
	List          []string          `flag:"list" usage:"list"`
	NestedFlags   Object            `flag:"nested" usage:"nested"`
	SquashedFlags Object            `flag:"-" usage:"nested"`
	Map           map[string]string `flag:"map" usage:"map"`

	// Globals `flag:"-" usage:"global flags through composition"`
}

func execute(ctx context.Context, flags Flags) error {
	data, err := json.MarshalIndent(flags, "", "  ")
	if err != nil {
		return err
	}
	println("flags:", string(data))
	println("remaining args:", strings.Join(struct_flags.GetRemainingArgs(ctx), " "))
	return nil
}
