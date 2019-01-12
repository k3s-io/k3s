package generator

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"k8s.io/gengo/args"
	"k8s.io/gengo/examples/deepcopy-gen/generators"
	"k8s.io/gengo/generator"
	gengotypes "k8s.io/gengo/types"
)

var (
	blackListTypes = map[string]bool{
		"schema":     true,
		"resource":   true,
		"collection": true,
	}
	underscoreRegexp = regexp.MustCompile(`([a-z])([A-Z])`)
)

type fieldInfo struct {
	Name string
	Type string
}

func getGoType(field types.Field, schema *types.Schema, schemas *types.Schemas) string {
	return getTypeString(field.Nullable, field.Type, schema, schemas)
}

func getTypeString(nullable bool, typeName string, schema *types.Schema, schemas *types.Schemas) string {
	switch {
	case strings.HasPrefix(typeName, "reference["):
		return "string"
	case strings.HasPrefix(typeName, "map["):
		return "map[string]" + getTypeString(false, typeName[len("map["):len(typeName)-1], schema, schemas)
	case strings.HasPrefix(typeName, "array["):
		return "[]" + getTypeString(false, typeName[len("array["):len(typeName)-1], schema, schemas)
	}

	name := ""

	switch typeName {
	case "base64":
		return "string"
	case "json":
		return "interface{}"
	case "boolean":
		name = "bool"
	case "float":
		name = "float64"
	case "int":
		name = "int64"
	case "multiline":
		return "string"
	case "masked":
		return "string"
	case "password":
		return "string"
	case "date":
		return "string"
	case "string":
		return "string"
	case "enum":
		return "string"
	case "intOrString":
		return "intstr.IntOrString"
	case "dnsLabel":
		return "string"
	case "dnsLabelRestricted":
		return "string"
	case "hostname":
		return "string"
	default:
		if schema != nil && schemas != nil {
			otherSchema := schemas.Schema(&schema.Version, typeName)
			if otherSchema != nil {
				name = otherSchema.CodeName
			}
		}

		if name == "" {
			name = convert.Capitalize(typeName)
		}
	}

	if nullable {
		return "*" + name
	}

	return name
}

func getTypeMap(schema *types.Schema, schemas *types.Schemas) map[string]fieldInfo {
	result := map[string]fieldInfo{}
	for name, field := range schema.ResourceFields {
		if strings.EqualFold(name, "id") {
			continue
		}
		result[field.CodeName] = fieldInfo{
			Name: name,
			Type: getGoType(field, schema, schemas),
		}
	}
	return result
}

func getResourceActions(schema *types.Schema, schemas *types.Schemas) map[string]types.Action {
	result := map[string]types.Action{}
	for name, action := range schema.ResourceActions {
		if action.Output != "" {
			if schemas.Schema(&schema.Version, action.Output) != nil {
				result[name] = action
			}
		} else {
			result[name] = action
		}
	}
	return result
}

func getCollectionActions(schema *types.Schema, schemas *types.Schemas) map[string]types.Action {
	result := map[string]types.Action{}
	for name, action := range schema.CollectionActions {
		if action.Output != "" {
			output := action.Output
			if action.Output == "collection" {
				output = strings.ToLower(schema.CodeName)
			}
			if schemas.Schema(&schema.Version, output) != nil {
				result[name] = action
			}
		} else {
			result[name] = action
		}
	}
	return result
}

func generateType(outputDir string, schema *types.Schema, schemas *types.Schemas) error {
	filePath := strings.ToLower("zz_generated_" + addUnderscore(schema.ID) + ".go")
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("type.template").
		Funcs(funcs()).
		Parse(strings.Replace(typeTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	return typeTemplate.Execute(output, map[string]interface{}{
		"schema":            schema,
		"structFields":      getTypeMap(schema, schemas),
		"resourceActions":   getResourceActions(schema, schemas),
		"collectionActions": getCollectionActions(schema, schemas),
	})
}

func generateLifecycle(external bool, outputDir string, schema *types.Schema, schemas *types.Schemas) error {
	filePath := strings.ToLower("zz_generated_" + addUnderscore(schema.ID) + "_lifecycle_adapter.go")
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("lifecycle.template").
		Funcs(funcs()).
		Parse(strings.Replace(lifecycleTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	importPackage := ""
	prefix := ""
	if external {
		parts := strings.Split(schema.PkgName, "/vendor/")
		importPackage = fmt.Sprintf("\"%s\"", parts[len(parts)-1])
		prefix = schema.Version.Version + "."
	}

	return typeTemplate.Execute(output, map[string]interface{}{
		"schema":        schema,
		"importPackage": importPackage,
		"prefix":        prefix,
	})
}

func generateController(external bool, outputDir string, schema *types.Schema, schemas *types.Schemas) error {
	filePath := strings.ToLower("zz_generated_" + addUnderscore(schema.ID) + "_controller.go")
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("controller.template").
		Funcs(funcs()).
		Parse(strings.Replace(controllerTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	importPackage := ""
	prefix := ""
	if external {
		parts := strings.Split(schema.PkgName, "/vendor/")
		importPackage = fmt.Sprintf("\"%s\"", parts[len(parts)-1])
		prefix = schema.Version.Version + "."
	}

	return typeTemplate.Execute(output, map[string]interface{}{
		"schema":        schema,
		"importPackage": importPackage,
		"prefix":        prefix,
	})
}

func generateScheme(external bool, outputDir string, version *types.APIVersion, schemas []*types.Schema) error {
	filePath := strings.ToLower("zz_generated_scheme.go")
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("scheme.template").
		Funcs(funcs()).
		Parse(strings.Replace(schemeTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	names := []string{}
	for _, schema := range schemas {
		if !external {
			names = append(names, schema.CodeName)
		}
		if schema.CanList(nil) == nil {
			names = append(names, schema.CodeName+"List")
		}
	}

	return typeTemplate.Execute(output, map[string]interface{}{
		"version": version,
		"schemas": schemas,
		"names":   names,
	})
}

func generateK8sClient(outputDir string, version *types.APIVersion, schemas []*types.Schema) error {
	filePath := strings.ToLower("zz_generated_k8s_client.go")
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("k8sClient.template").
		Funcs(funcs()).
		Parse(strings.Replace(k8sClientTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	return typeTemplate.Execute(output, map[string]interface{}{
		"version": version,
		"schemas": schemas,
	})
}

func generateClient(outputDir string, schemas []*types.Schema) error {
	template, err := template.New("client.template").
		Funcs(funcs()).
		Parse(clientTemplate)
	if err != nil {
		return err
	}

	output, err := os.Create(path.Join(outputDir, "zz_generated_client.go"))
	if err != nil {
		return err
	}
	defer output.Close()

	return template.Execute(output, map[string]interface{}{
		"schemas": schemas,
	})
}

func GenerateControllerForTypes(version *types.APIVersion, k8sOutputPackage string, nsObjs []interface{}, objs []interface{}) error {
	baseDir := args.DefaultSourceTree()
	k8sDir := path.Join(baseDir, k8sOutputPackage)

	if err := prepareDirs(k8sDir); err != nil {
		return err
	}

	schemas := types.NewSchemas()
	var controllers []*types.Schema

	for _, obj := range objs {
		schema, err := schemas.Import(version, obj)
		if err != nil {
			return err
		}
		controllers = append(controllers, schema)

		if err := generateController(true, k8sDir, schema, schemas); err != nil {
			return err
		}

		if err := generateLifecycle(true, k8sDir, schema, schemas); err != nil {
			return err
		}
	}

	for _, obj := range nsObjs {
		schema, err := schemas.Import(version, obj)
		if err != nil {
			return err
		}
		schema.Scope = types.NamespaceScope
		controllers = append(controllers, schema)

		if err := generateController(true, k8sDir, schema, schemas); err != nil {
			return err
		}

		if err := generateLifecycle(true, k8sDir, schema, schemas); err != nil {
			return err
		}
	}

	if err := deepCopyGen(baseDir, k8sOutputPackage); err != nil {
		return err
	}

	if err := generateK8sClient(k8sDir, version, controllers); err != nil {
		return err
	}

	if err := generateScheme(true, k8sDir, version, controllers); err != nil {
		return err
	}

	return gofmt(baseDir, k8sOutputPackage)
}

func Generate(schemas *types.Schemas, privateTypes map[string]bool, cattleOutputPackage, k8sOutputPackage string) error {
	baseDir := args.DefaultSourceTree()
	cattleDir := path.Join(baseDir, cattleOutputPackage)
	k8sDir := path.Join(baseDir, k8sOutputPackage)

	if cattleOutputPackage == "" {
		cattleDir = ""
	}

	if err := prepareDirs(cattleDir, k8sDir); err != nil {
		return err
	}

	var controllers []*types.Schema

	var cattleClientTypes []*types.Schema
	for _, schema := range schemas.Schemas() {
		if blackListTypes[schema.ID] {
			continue
		}

		_, privateType := privateTypes[schema.ID]

		if cattleDir != "" {
			if err := generateType(cattleDir, schema, schemas); err != nil {
				return err
			}
		}

		if privateType ||
			(contains(schema.CollectionMethods, http.MethodGet) &&
				!strings.HasPrefix(schema.PkgName, "k8s.io") &&
				!strings.Contains(schema.PkgName, "/vendor/")) {
			controllers = append(controllers, schema)
			if err := generateController(false, k8sDir, schema, schemas); err != nil {
				return err
			}
			if err := generateLifecycle(false, k8sDir, schema, schemas); err != nil {
				return err
			}
		}

		if !privateType {
			cattleClientTypes = append(cattleClientTypes, schema)
		}
	}

	if cattleDir != "" {
		if err := generateClient(cattleDir, cattleClientTypes); err != nil {
			return err
		}
	}

	if len(controllers) > 0 {
		if err := deepCopyGen(baseDir, k8sOutputPackage); err != nil {
			return err
		}

		if err := generateK8sClient(k8sDir, &controllers[0].Version, controllers); err != nil {
			return err
		}

		if err := generateScheme(false, k8sDir, &controllers[0].Version, controllers); err != nil {
			return err
		}
	}

	if err := gofmt(baseDir, k8sOutputPackage); err != nil {
		return err
	}

	if cattleOutputPackage != "" {
		return gofmt(baseDir, cattleOutputPackage)
	}

	return nil
}

func prepareDirs(dirs ...string) error {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, file := range files {
			if strings.HasPrefix(file.Name(), "zz_generated") {
				if err := os.Remove(path.Join(dir, file.Name())); err != nil {
					return errors.Wrapf(err, "failed to delete %s", path.Join(dir, file.Name()))
				}
			}
		}
	}

	return nil
}

func gofmt(workDir, pkg string) error {
	cmd := exec.Command("goimports", "-w", "-l", "./"+pkg)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func deepCopyGen(workDir, pkg string) error {
	arguments := &args.GeneratorArgs{
		InputDirs:          []string{pkg},
		OutputBase:         workDir,
		OutputPackagePath:  pkg,
		OutputFileBaseName: "zz_generated_deepcopy",
		GoHeaderFilePath:   "/dev/null",
		GeneratedBuildTag:  "ignore_autogenerated",
	}

	return arguments.Execute(
		generators.NameSystems(),
		generators.DefaultNameSystem(),
		func(context *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
			packageParts := strings.Split(pkg, "/")
			return generator.Packages{
				&generator.DefaultPackage{
					PackageName: packageParts[len(packageParts)-1],
					PackagePath: pkg,
					HeaderText:  []byte{},
					GeneratorFunc: func(c *generator.Context) []generator.Generator {
						return []generator.Generator{
							//&noInitGenerator{
							generators.NewGenDeepCopy(arguments.OutputFileBaseName, pkg, nil, true, true),
							//},
						}
					},
					FilterFunc: func(c *generator.Context, t *gengotypes.Type) bool {
						if t.Name.Package != pkg {
							return false
						}

						if isObjectOrList(t) {
							t.SecondClosestCommentLines = append(t.SecondClosestCommentLines,
								"+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object")
						}

						return true
					},
				},
			}
		})
}

func isObjectOrList(t *gengotypes.Type) bool {
	for _, member := range t.Members {
		if member.Embedded && (member.Name == "ObjectMeta" || member.Name == "ListMeta") {
			return true
		}
		if member.Embedded && isObjectOrList(member.Type) {
			return true
		}
	}

	return false
}
