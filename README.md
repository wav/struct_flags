# struct_flags

Yet another flags package. This one defines flags using go's "flags" using structs and integrates validators from "github.com/asaskevich/govalidator".

# Features

- Parse flags using the "go" package into a struct
- A command has the form `func (context.Context, FlagsType) error`
- Flags can have validation tags
- Flags can be read from the environment if specified
- Structs can be nested and optionally squashed
- `my_util @argfile.txt --string=1` will read flags from an *ArgFile* (a json object)
- *ArgFile* supports variable replacement. eg. for `a=1`, `--flag=$a` will become `--flag=1`

# Missing

- Positional Arguments, if ever implemented, will follow flags.

# Order of arguments

`my_util [command] [flags] [args]`

# Sample

```go
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
	c := struct_flags.NewCommand("print-args", Flags{},"print the provided arguments if they validate ok", execute)
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

```

## Output

```bash
bash:my_util$ ./my_util
usage: ./my_util [command] [args]

  print-args      print the provided arguments if they validate ok
  more
```

```bash
bash:my_util$ ./my_util print-args -filepath=
invalid value "" for flag -filepath: validation failed for rule 'required'
```

```bash
bash:my_util$ BOOL=true ./my_util print-args -filepath=$(pwd)/main.go -map has_filter=true,has_env=yes -list=1,2 -map=key=value -list=3 remainder1 remainder2 --remainder3

flags: {
  "String": "",
  "Filepath": "/Users/wassim/Repositories/src/git.wav.im/wav/struct_flags/cmd/my_util/main.go",
  "Int": 0,
  "Bool": true,
  "List": [
    "1",
    "2",
    "3"
  ],
  "NestedFlags": {
    "String1": "",
    "String2": ""
  },
  "SquashedFlags": {
    "String1": "",
    "String2": ""
  },
  "Map": {
    "has_env": "yes",
    "has_filter": "true",
    "key": "value"
  }
}
remaining args: remainder1 remainder2 --remainder3
```

The above command can also be put in an *ArgsFile*

```argsfile.txt
{
	"command": ["print-args"],
	"env": [
		"bool=$BOOL",
		"BOOL=$bool"
	],
	"args": [
		"-filepath=$PWD/main.go",
		"-map", "has_filter=true,has_env=yes",
		"-list=1,2",
		"-map=key=value",
		"-list=3",
		"remainder1",
		"remainder2",
		"--remainder3"
	]
}
```

```bash
bash:my_util$ BOOL=true ./my_util @argsfile.txt
```

