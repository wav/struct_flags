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
- Subcommands

# Missing

- Positional Arguments. If ever implemented, they will follow flags.

# Order of arguments

`my_util [command] [flags] [args]`

# Sample

```go
package main

import (
	"context"
	"github/wav/struct_flags"
	"os"
	"encoding/json"
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

```

## Output

```bash
bash:my_util$ ./my_util
usage: ./my_util [command] [args]

  print-args      print the provided arguments if they validate ok
```

```bash
bash:my_util$ ./my_util print-args -filepath=
flag provided but it does not satisfy the rule 'required': -filepath
```

```bash
bash:my_util$ BOOL=true ./my_util print-args -filepath=$(pwd)/main.go -map has_filter=true,has_env=yes -list=1,2 -map=key=value -list=3

{
  "String": "",
  "Filepath": "/go/src/github.com/wav/struct_flags/cmd/my_util/main.go",
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

```

The above command can also be put in an *ArgsFile*

```argsfile.txt
{
	"command": ["print-args"],
	"env": ["BOOL=true"],
	"args": [
		"-filepath=$PWD/main.go",
		"-map", "has_filter=true,has_env=yes",
		"-list=1,2",
		"-map=key=value",
		"-list=3"
	]
}
```

```bash
bash:my_util$ BOOL=true ./my_util @argsfile.txt
```

