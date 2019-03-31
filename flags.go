package struct_flags

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"gopkg.in/go-playground/validator.v9"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

type Commands []ICommand

type ICommand interface {
	Name() string
	Usage() string
}

type Command interface {
	ICommand
	// DefaultFlags is a prefilled instance of a struct type that flag parsing will populate
	DefaultFlags() (flags interface{})
	// Prepare validates and does any necessary transformations before Execute is called.
	// [flags] must be of the same type returned by DefaultFlags
	PrepareFlags(flags interface{}) error
	Execute(ctx context.Context, flags interface{}) error
}

type CommandGroup interface {
	ICommand
	Commands() Commands
}

type ArgFile struct {
	Command []string `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env"`
}

func NewCommand(name, usage string, defaultFlags, prepareFlags, execute interface{}) Command {
	if name == "" {
		panic("'name' name must be provided")
	}
	if execute == nil {
		panic("'execute' must be provided")
	}
	c := command{
		name:         name,
		usage:        usage,
		defaultFlags: defaultFlags,
		prepareFlags: PrepareStructFields,
	}

	// by default do nothing
	if c.prepareFlags == nil {
		c.prepareFlags = func(a interface{}) error {
			return nil
		}
	}

	// Prepare

	if prepareFlags != nil {
		prepareT := reflect.TypeOf(prepareFlags)
		if prepareT.Kind() != reflect.Func || prepareT.NumIn() != 1 || prepareT.NumOut() != 1 || !prepareT.Out(0).Implements(errorType) || prepareT.In(0) != reflect.TypeOf(defaultFlags) {
			s := "nil"
			if defaultFlags != nil {
				s = reflect.TypeOf(defaultFlags).String()
			}
			panic(fmt.Errorf("prepareFlags is not a function of type 'func (%s) error'", s))
		}
		preparer := c.prepareFlags
		c.prepareFlags = func(arg interface{}) error {
			if err := preparer(arg); err != nil {
				return err
			}
			if err := reflect.ValueOf(prepareFlags).Call([]reflect.Value{reflect.ValueOf(arg)})[0].Interface(); err != nil {
				return err.(error)
			}
			return nil
		}
	}

	// Execute

	execT := reflect.TypeOf(execute)
	if execT.Kind() != reflect.Func || execT.NumIn() != 2 || execT.NumOut() != 1 || !execT.Out(0).Implements(errorType) || !execT.In(0).Implements(contextType) || execT.In(1) != reflect.TypeOf(defaultFlags) {
		s := "nil"
		if defaultFlags != nil {
			s = reflect.TypeOf(defaultFlags).String()
		}
		panic(fmt.Errorf("execute, for command '%s', is not a function of type 'func (context.Context, %s) error'", name, s))
	}
	c.execute = func(ctx context.Context, arg interface{}) error {
		if err := reflect.ValueOf(execute).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(arg)})[0].Interface(); err != nil {
			return err.(error)
		}
		return nil
	}
	return c
}

func NewCommandGroup(name, usage string, commands ...ICommand) CommandGroup {
	return &commandGroup{
		name:     name,
		usage:    usage,
		commands: commands,
	}
}

var PrepareStructFields = defaultPrepareStructFields

type usage struct {
	Description string
}

func (u usage) Error() string {
	return u.Description
}

var parentCommandsKey = &struct{}{}

func getParentCommands(ctx context.Context) []string {
	value := ctx.Value(parentCommandsKey)
	if value == nil {
		return nil
	}
	if parents, ok := value.([]string); ok {
		return parents
	}
	return nil
}

func withParentCommands(ctx context.Context, parents []string) context.Context {
	return context.WithValue(ctx, parentCommandsKey, parents)
}

var argFileKey = &struct{}{}

func getArgFile(ctx context.Context) *ArgFile {
	value := ctx.Value(argFileKey)
	if value == nil {
		return nil
	}
	if argFile, ok := value.(*ArgFile); ok {
		return argFile
	}
	return nil
}

func withArgFile(ctx context.Context, argFile *ArgFile) context.Context {
	return context.WithValue(ctx, argFileKey, argFile)
}

func (cs Commands) Run(ctx context.Context, args []string) error {
	parentCommands := getParentCommands(ctx)
	minArgs := len(parentCommands) + 2
	if len(args) < minArgs {
		return cs.usage(args)
	}
	currentCommandName := strings.ToLower(args[len(parentCommands)+1])

	// Argfile requested
	if strings.HasPrefix(currentCommandName, "@") && getArgFile(ctx) == nil {
		data, err := ioutil.ReadFile(currentCommandName[1:])
		if err != nil {
			return fmt.Errorf("could not open @argfile, err: %s", err.Error())
		}
		var argFile ArgFile
		if err := json.Unmarshal(data, &argFile); err != nil {
			return fmt.Errorf("could not read @argfile, err: %s", err.Error())
		}
		ctx = withParentCommands(ctx, append(parentCommands, argFile.Command...))
		ctx = withArgFile(ctx, &argFile)
		for _, env := range argFile.Env {
			kv := strings.SplitN(env, "=", 2)
			if err := os.Setenv(kv[0], os.ExpandEnv(kv[1])); err != nil {
				return fmt.Errorf("failed to apply environment variable %s from @argfile", err.Error())
			}
		}
		var mergedArgs []string
		mergedArgs = append(mergedArgs, args[:len(parentCommands)+1]...)
		mergedArgs = append(mergedArgs, argFile.Command...)
		fileArgs := append([]string{}, argFile.Args...)
		for i, arg := range fileArgs {
			fileArgs[i] = os.ExpandEnv(arg)
		}
		mergedArgs = append(mergedArgs, fileArgs...)
		mergedArgs = append(mergedArgs, args[len(parentCommands)+2:]...)
		return cs.Run(ctx, mergedArgs)
	}

	var command Command
	for _, c := range cs {
		if strings.ToLower(c.Name()) == currentCommandName {
			switch t := c.(type) {
			case Command:
				command = t
				break
			case CommandGroup:
				return t.Commands().Run(withParentCommands(ctx, append(parentCommands, currentCommandName)), args)
			}
		}
	}
	if command == nil || command.Name() == "" {
		return cs.usage(args)
	}
	flags := command.DefaultFlags()
	arg, err := parseCommandFlags(flags, args[minArgs:])
	if err != nil {
		return err
	}
	if err := command.PrepareFlags(arg); err != nil {
		if len(args) == minArgs {
			// the default flags values fail
			_, err := parseCommandFlags(flags, []string{"--help"})
			return err
		}
		switch verr := err.(type) {
		case validator.ValidationErrors:
			var errs []string
			for _, ferr := range verr {
				field, ok := getStructFieldForError(ferr, arg)
				if !ok {
					errs = append(errs, validator.ValidationErrors{ferr}.Error())
					continue
				}
				flagName := field.Tag.Get("flag")
				rule := ferr.Tag()
				if ferr.Param() != "" {
					rule += "=" + ferr.Param()
				}
				// Write a similar message to 'flags', eg. 'flag provided but not defined: -a'
				message := fmt.Sprintf("flag provided but it does not satisfy the rule '%s': -%s %s", rule, flagName, fmt.Sprint(ferr.Value()))
				errs = append(errs, message)
			}
			return errors.New(strings.Join(errs, "\n"))
		}
		return err
	}
	return command.Execute(ctx, arg)
}

func (cs Commands) usage(args []string) usage {
	desc := "usage: " + args[0] + " [command] [args]\n\n"
	nameWidth := 0
	for _, c := range cs {
		if c.Name() == "" {
			panic(reflect.TypeOf(c).String() + " has no ICommand.Name()")
		}
		if l := len(c.Name()); l > nameWidth {
			nameWidth = l
		}
	}
	nameWidth = nameWidth + nameWidth%4 + 4
	for _, c := range cs {
		desc += "  "
		if c.Usage() != "" {
			desc += c.Name() + strings.Repeat(" ", nameWidth-len(c.Name()))
			desc += c.Usage()
		} else {
			desc += c.Name()
		}
		desc += "\n"
	}
	return usage{Description: desc}
}

// commandArgs = args[2:]
func parseCommandFlags(commandFlags interface{}, commandArgs []string) (updatedFlags interface{}, err error) {
	if commandFlags != nil {
		ft := reflect.TypeOf(commandFlags)
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		v := reflect.New(ft)
		fs := NewFlagSet(commandFlags)
		err = handleError(func() error {
			switch v.Elem().Kind() {
			case reflect.Slice:
				updatedFlags = commandArgs
			default:
				_, err := fs.UnmarshalFlags(commandArgs, v.Interface())
				updatedFlags = reflect.Indirect(v).Interface()
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return
}

func handleError(f func() error) error {
	switch err := f().(type) {
	case nil:
		return nil
	case flagConfigError:
		panic(err)
	case error:
		return err
	default:
		panic("unreachable")
	}
}

func getStructFieldForError(e validator.FieldError, v interface{}) (reflect.StructField, bool) {
	cursor := reflect.ValueOf(v)
	path := strings.Split(e.StructNamespace(), ".")[1:]
	for i, name := range path {
		last := i == len(path)-1
		if last {
			return cursor.Type().FieldByName(name)
		} else {
			cursor = cursor.FieldByName(name)
		}
	}
	return reflect.StructField{}, false
}

type flagConfigError struct {
	err string
	v   reflect.Value
}

func (c flagConfigError) Error() string {

	return c.err + ", for " + describeValue(c.v)
}

func describeValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Func:
		f := runtime.FuncForPC(v.Pointer())
		fname, line := f.FileLine(v.Pointer())
		return fmt.Sprintf("%s of type %s in %s:%d", f.Name(), v.Type().String(), fname, line)
	default:
		return v.String()
	}
}

type FlagSet interface {
	UnmarshalFlags(argsAndFlags []string, a interface{}) (args []string, err error)
}

func NewFlagSet(defaults interface{}) FlagSet {
	v := reflect.Indirect(reflect.ValueOf(defaults))
	if v.Kind() != reflect.Struct && v.Kind() != reflect.Slice {
		panic("expected struct or slice type, got: " + v.Type().String())
	}
	return flagSet{
		defaults: v.Interface(),
	}
}

type flagSet struct {
	defaults interface{}
}

type flagInfo struct {
	name  string
	usage string
	env   string
	set   func()
}

func (fi flagInfo) fullUsage() string {
	if fi.env == "" {
		return fi.usage
	}
	return fi.usage + " (env \"" + fi.env + "\")"
}

func (fi flagInfo) readEnv(valuePtr interface{}) bool {
	if fi.env == "" {
		return false
	}
	envValue, ok := os.LookupEnv(fi.env)
	if !ok {
		return false
	}
	value := reflect.ValueOf(valuePtr).Elem()
	switch value.Kind() {
	case reflect.String:
		value.SetString(envValue)
		return true
	case reflect.Bool:
		truthy, err := strconv.ParseBool(envValue)
		if err != nil {
			return false
		}
		value.SetBool(truthy)
		return true
	case reflect.Int:
		integer, err := strconv.ParseInt(envValue, 10, 64)
		if err != nil {
			return false
		}
		value.SetInt(integer)
		return true
	}
	return false
}

func readFlagInfo(t reflect.Type, prefix string, i int) (*flagInfo, bool) {
	f := t.Field(i)
	tag := f.Tag
	flagTag := strings.Split(tag.Get("flag"), ",")
	if flagTag[0] == "" {
		return nil, false
	}
	info := flagInfo{
		name:  prefix + flagTag[0],
		usage: tag.Get("usage"),
		env:   tag.Get("env"),
	}
	return &info, true
}

func (s flagSet) UnmarshalFlags(args []string, a interface{}) ([]string, error) {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	defaults := reflect.ValueOf(s.defaults)
	focus := reflect.ValueOf(a)
	var flags []flagInfo
	seen := map[reflect.Type]*struct{}{}
	flags = collectStructFlags(fs, flags, "", defaults, focus, seen)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	// build leaves first
	for i := len(flags) - 1; i >= 0; i-- {
		f := flags[i]
		f.set()
	}
	return fs.Args(), nil
}

func collectStructFlags(fs *flag.FlagSet, collected []flagInfo, prefix string, defaults, focus reflect.Value, seen map[reflect.Type]*struct{}) []flagInfo {
	if focus.Kind() != reflect.Ptr || focus.Elem().Type() != defaults.Type() {
		panic("expected *" + defaults.String() + ", got: " + focus.String())
	}
	if _, ok := seen[focus.Type()]; ok {
		panic("cycle in flag types found for type: " + focus.String())
	}
	seen[focus.Type()] = nil
	for i := 0; i < defaults.NumField(); i++ {
		info, ok := readFlagInfo(defaults.Type(), prefix, i)
		if !ok {
			continue
		}
		fieldValue := focus.Elem().Field(i)
		switch fieldValue.Kind() {
		case reflect.String:
			df := defaults.Field(i).String()
			info.readEnv(&df)
			s := fs.String(info.name, df, info.fullUsage())
			info.set = func() {
				fieldValue.SetString(*s)
			}
		case reflect.Bool:
			df := defaults.Field(i).Bool()
			info.readEnv(&df)
			b := fs.Bool(info.name, df, info.fullUsage())
			info.set = func() {
				fieldValue.SetBool(*b)
			}
		case reflect.Int:
			df := defaults.Field(i).Int()
			info.readEnv(&df)
			i := fs.Int(info.name, int(df), info.fullUsage())
			info.set = func() {
				fieldValue.SetInt(int64(*i))
			}
		case reflect.Map:
			if fieldValue.Type().Key().Kind() != reflect.String {
				continue
			}
			sm := stringMap{}
			fs.Var(&sm, info.name, info.fullUsage())
			info.set = func() {
				m := map[string]string{}
				for _, e := range sm {
					m[e.key] = e.value
				}
				fieldValue.Set(reflect.ValueOf(m))
			}
		case reflect.Slice:
			arr := stringArray{}
			fs.Var(&arr, info.name, info.fullUsage())
			info.set = func() {
				fieldValue.Set(reflect.ValueOf(arr))
			}
		case reflect.Struct, reflect.Interface:
			prefix := ""
			if info.name != "-" {
				prefix = info.name + "."
			}
			collected = collectStructFlags(fs, collected, prefix, defaults.Field(i), fieldValue.Addr(), seen)
			continue
		default:
			continue
		}
		collected = append(collected, *info)
	}
	delete(seen, focus.Type())
	return collected
}

type stringArray []string

func (sa stringArray) String() string {
	return strings.Join(sa, ",")
}

func (sa *stringArray) Set(v string) error {
	*sa = append(*sa, strings.Split(v, ",")...)
	return nil
}

func (sa stringArray) Get() interface{} {
	return sa
}

type stringMapEntry struct {
	key   string
	value string
}

type stringMap []stringMapEntry

func (sm stringMap) String() string {
	var entries []string
	for _, e := range sm {
		entries = append(entries, e.key+"="+e.value)
	}
	return strings.Join(entries, ",")
}

func (sm *stringMap) Set(v string) error {
	entries := strings.Split(v, ",")
	for _, e := range entries {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 1 {
			*sm = append(*sm, stringMapEntry{key: kv[0]})
		} else {
			*sm = append(*sm, stringMapEntry{key: kv[0], value: kv[1]})
		}
	}
	return nil
}

func (sm stringMap) Get() interface{} {
	return sm
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

type command struct {
	name         string
	usage        string
	defaultFlags interface{}
	prepareFlags func(interface{}) error
	execute      func(context.Context, interface{}) error
}

func (c command) Name() string {
	return c.name
}

func (c command) Usage() string {
	return c.usage
}

func (c command) DefaultFlags() interface{} {
	return c.defaultFlags
}

func (c command) PrepareFlags(arg interface{}) error {
	return c.prepareFlags(arg)
}

func (c command) Execute(ctx context.Context, arg interface{}) error {
	return c.execute(ctx, arg)
}

type commandGroup struct {
	name     string
	usage    string
	commands Commands
}

func (c commandGroup) Name() string {
	return c.name
}

func (c commandGroup) Usage() string {
	return c.usage
}

func (c commandGroup) Commands() Commands {
	return c.commands
}
