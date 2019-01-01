package builder

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"github.com/urfave/cli"
)

var (
	caseRegexp = regexp.MustCompile("([a-z])([A-Z])")
)

type clirunnable interface {
	Run(app *cli.Context) error
}

type customizer interface {
	Customize(cmd *cli.Command)
}

type fieldInfo struct {
	FieldType  reflect.StructField
	FieldValue reflect.Value
}

func fields(obj interface{}) []fieldInfo {
	ptrValue := reflect.ValueOf(obj)
	objValue := ptrValue.Elem()

	var result []fieldInfo

	for i := 0; i < objValue.NumField(); i++ {
		fieldType := objValue.Type().Field(i)
		if fieldType.Anonymous && fieldType.Type.Kind() == reflect.Struct {
			result = append(result, fields(objValue.Field(i).Addr().Interface())...)
		} else if !fieldType.Anonymous {
			result = append(result, fieldInfo{
				FieldValue: objValue.Field(i),
				FieldType:  objValue.Type().Field(i),
			})
		}
	}

	return result
}

func Command(obj interface{}, usage, usageText, description string) cli.Command {
	slices := map[string]reflect.Value{}
	maps := map[string]reflect.Value{}
	ptrValue := reflect.ValueOf(obj)
	objValue := ptrValue.Elem()

	c := cli.Command{
		Name:                   strings.ToLower(strings.Replace(objValue.Type().Name(), "Command", "", 1)),
		Usage:                  usage,
		UsageText:              usageText,
		Description:            description,
		UseShortOptionHandling: true,
		SkipArgReorder:         true,
	}

	for _, info := range fields(obj) {
		defMessage := ""
		fieldType := info.FieldType
		v := info.FieldValue

		switch fieldType.Type.Kind() {
		case reflect.Int:
			flag := cli.IntFlag{
				Name:        name(fieldType.Name),
				Usage:       fieldType.Tag.Get("desc"),
				EnvVar:      fieldType.Tag.Get("env"),
				Destination: (*int)(unsafe.Pointer(v.Addr().Pointer())),
			}
			defValue := fieldType.Tag.Get("default")
			if defValue != "" {
				n, err := strconv.Atoi(defValue)
				if err != nil {
					panic("bad default " + defValue + " on field " + fieldType.Name)
				}
				flag.Value = n
			}
			c.Flags = append(c.Flags, flag)
		case reflect.String:
			flag := cli.StringFlag{
				Name:        name(fieldType.Name),
				Usage:       fieldType.Tag.Get("desc"),
				Value:       fieldType.Tag.Get("default"),
				EnvVar:      fieldType.Tag.Get("env"),
				Destination: (*string)(unsafe.Pointer(v.Addr().Pointer())),
			}
			c.Flags = append(c.Flags, flag)
		case reflect.Slice:
			slices[name(fieldType.Name)] = v
			defMessage = " "
			fallthrough
		case reflect.Map:
			if defMessage == "" {
				maps[name(fieldType.Name)] = v
				defMessage = " "
			}
			flag := cli.StringSliceFlag{
				Name:   name(fieldType.Name),
				Usage:  fieldType.Tag.Get("desc") + defMessage,
				EnvVar: fieldType.Tag.Get("env"),
				Value:  &cli.StringSlice{},
			}
			c.Flags = append(c.Flags, flag)
		case reflect.Bool:
			flag := cli.BoolFlag{
				Name:        name(fieldType.Name),
				Usage:       fieldType.Tag.Get("desc"),
				EnvVar:      fieldType.Tag.Get("env"),
				Destination: (*bool)(unsafe.Pointer(v.Addr().Pointer())),
			}
			c.Flags = append(c.Flags, flag)
		default:
			panic("Unknown kind on field " + fieldType.Name + " on " + objValue.Type().Name())
		}
	}

	if run, ok := obj.(clirunnable); ok {
		c.Action = run.Run
	} else {
		panic(fmt.Sprintf("failed to find Action function for %T", obj))
	}

	cust, ok := obj.(customizer)
	if ok {
		cust.Customize(&c)
	}

	return c
}

func name(name string) string {
	parts := strings.Split(name, "_")
	i := len(parts) - 1
	name = caseRegexp.ReplaceAllString(parts[i], "$1-$2")
	name = strings.ToLower(name)
	result := append([]string{name}, parts[0:i]...)
	for i := 0; i < len(result); i++ {
		result[i] = strings.ToLower(result[i])
	}
	return strings.Join(result, ",")
}
