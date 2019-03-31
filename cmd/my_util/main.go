package main

import (
	"context"
	"encoding/json"
	"github.com/wav/struct_flags"
	"os"
)

type Object struct {
	String1 string `flag:"string1" usage:"string1 if squashed, otherwise nested.string1"`
	String2 string `flag:"string2" usage:"string2 if squashed, otherwise nested.string2"`
}

type Flags struct {
	String        string            `flag:"string" usage:"string"`
	Filepath      string            `flag:"filepath" usage:"filepath" validate:"required,file=exists,file=absolute"`
	Int           int               `flag:"int" usage:"int"`
	Bool          bool              `flag:"bool" env:"BOOL" usage:"bool"`
	List          []string          `flag:"list" usage:"list"`
	NestedFlags   Object            `flag:"nested" usage:"nested"`
	SquashedFlags Object            `flag:"-" usage:"nested"`
	Map           map[string]string `flag:"map" usage:"map"`
}

var commands struct_flags.Commands

func init() {
	defaultFlags := Flags{}
	c := struct_flags.NewCommand("print-args", "print the provided arguments if they validate ok", defaultFlags, nil, execute)
	commands = append(commands, c)
}

func execute(_ context.Context, flags Flags) error {
	data, err := json.MarshalIndent(flags, "", "  ")
	if err != nil {
		return err
	}
	println(string(data))
	return nil
}

func main() {
	if err := commands.Run(context.TODO(), os.Args); err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
