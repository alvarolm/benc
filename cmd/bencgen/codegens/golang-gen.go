package codegens

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alvarolm/benc/cmd/bencgen/lexer"
	"github.com/alvarolm/benc/cmd/bencgen/parser"
	"github.com/alvarolm/benc/cmd/bencgen/utils"
)

type GoContainerStmt struct {
	PublicName  string
	PrivateName string

	DefaultName string

	Fields      []parser.Field
	ReservedIDs []uint16
	Comment     string
}

type GoEnumStmt struct {
	PublicName  string
	PrivateName string

	DefaultName string

	Values        []string
	ValueComments []string
	Comment       string
}

type GoField struct {
	ID uint16

	PublicName  string
	PrivateName string
	DefaultName string

	Type    *parser.Type
	Comment string
}

// docComment formats a (possibly multi-line) schema comment as Go line
// comments, each prefixed with indent. Returns "" for an empty comment.
func docComment(comment, indent string) string {
	if comment == "" {
		return ""
	}
	var sb strings.Builder
	for _, line := range strings.Split(comment, "\n") {
		sb.WriteString(indent + "// " + line + "\n")
	}
	return sb.String()
}

func (f *GoField) AppendUnsafeIfPresent() string {

	return f.Type.AppendUnsafeIfPresent()
}

func (f *GoField) AppendReturnCopyIfPresent() string {
	return f.Type.AppendReturnCopyIfPresent()
}

type GoGen struct {
	file string

	enumDecls      []string
	containerDecls []string
	customMap      map[string]*parser.CustomStmt

	varMap map[string]string

	importedPackages          []string
	importedEnumsOrContainers map[string]string

	plainGen   bool
	defineStmt *parser.DefineStmt

	// currently generated...

	field         GoField
	enumStmt      GoEnumStmt
	containerStmt GoContainerStmt
	customStmt    *parser.CustomStmt
}

func NewGoGen(file string) *GoGen {
	return &GoGen{
		file:                      file,
		importedEnumsOrContainers: make(map[string]string),
		customMap:                 make(map[string]*parser.CustomStmt),
	}
}

func (g *GoGen) File() string {
	return g.file
}

func (*GoGen) Lang() GenLang {
	return GoGenLang
}

func (g *GoGen) IsEnum(externalStructure string) bool {
	return slices.Contains(g.enumDecls, externalStructure)
}

func (g *GoGen) IsContainer(externalStructure string) bool {
	return slices.Contains(g.containerDecls, externalStructure)
}

func (g *GoGen) IsCustom(externalStructure string) bool {
	_, ok := g.customMap[externalStructure]
	return ok
}

func lastPathSegment(path string) string {
	split := strings.Split(path, "/")
	return split[len(split)-1]
}

// resolveCustomTypes annotates t (and its child/key types) with the resolved
// metadata for any custom type it references. Must run before
// adjustExternalStructureToImports so ExternalStructure is still the raw name.
func (g *GoGen) resolveCustomTypes(t *parser.Type) {
	if t == nil {
		return
	}

	if t.IsAnExternalStructure() {
		if stmt, ok := g.customMap[t.ExternalStructure]; ok {
			t.IsCustom = true
			if stmt.IsAlias {
				public := utils.ToUpper(stmt.Name)
				priv := utils.ToLower(stmt.Name)
				t.CustomGoType = public
				t.CustomSizeFn = priv + "Size"
				t.CustomMarshalFn = priv + "Marshal"
				t.CustomUnmarshalFn = priv + "Unmarshal"
				t.CustomWireTag = g.mapTokenTypeToBgenimplType(stmt.BaseType)
			} else {
				alias := lastPathSegment(stmt.FuncsPath)
				public := utils.ToUpper(stmt.Name)
				t.CustomGoType = stmt.GoType
				t.CustomSizeFn = alias + ".Size" + public
				t.CustomMarshalFn = alias + ".Marshal" + public
				t.CustomUnmarshalFn = alias + ".Unmarshal" + public
				t.CustomWireTag = "Bytes"
			}
		}
		return
	}

	g.resolveCustomTypes(t.MapKeyType)
	g.resolveCustomTypes(t.ChildType)
}

func (g *GoGen) AddCustomDecls(stmts []*parser.CustomStmt) {
	for _, stmt := range stmts {
		g.customMap[stmt.Name] = stmt

		// Form B references an external codec package (and optionally the type's
		// own package); Form A is fully local and needs no imports.
		if stmt.IsAlias {
			continue
		}
		for _, pkg := range []string{stmt.FuncsPath, stmt.ImportPath} {
			if pkg != "" && !slices.Contains(g.importedPackages, pkg) {
				g.importedPackages = append(g.importedPackages, pkg)
			}
		}
	}
}

func (g *GoGen) SetCustomStatement(stmt *parser.CustomStmt) {
	g.customStmt = stmt
}

func (g *GoGen) ForEachCtrFields(f func(i int)) {
	for i, field := range g.containerStmt.Fields {
		g.field = GoField{
			ID: field.ID,

			PublicName:  utils.ToUpper(field.Name),
			PrivateName: utils.ToLower(field.Name),
			DefaultName: field.Name,

			Type:    field.Type,
			Comment: field.Comment,
		}
		f(i)
	}
}

func (g *GoGen) ForEachEnumValues(f func(i int, value string)) {
	for i, value := range g.enumStmt.Values {
		f(i, utils.ToUpper(value))
	}
}

func (g *GoGen) HasPackageDefined() bool {
	return g.defineStmt != nil
}

func (g *GoGen) SetVarMap(varMap map[string]string) {
	g.varMap = varMap
}

func (g *GoGen) SetDefineStatement(stmt *parser.DefineStmt) {
	g.defineStmt = stmt
}

func (g *GoGen) SetEnumStatement(stmt *parser.EnumStmt) {
	g.enumStmt = GoEnumStmt{
		PublicName:  utils.ToUpper(stmt.Name),
		PrivateName: utils.ToLower(stmt.Name),

		DefaultName:   stmt.Name,
		Values:        stmt.Values,
		ValueComments: stmt.ValueComments,
		Comment:       stmt.Comment,
	}
}

func (g *GoGen) adjustExternalStructureToImports(t *parser.Type) {
	if t.IsCustom {
		return
	}

	if t.IsAnExternalStructure() {
		if replacement, ok := g.importedEnumsOrContainers[t.ExternalStructure]; ok {
			t.ExternalStructure = replacement
		}
		return
	}

	if t.IsArray {
		g.adjustExternalStructureToImports(t.ChildType)
		return
	}

	if t.IsMap {
		g.adjustExternalStructureToImports(t.MapKeyType)
		g.adjustExternalStructureToImports(t.ChildType)
		return
	}
}

func (g *GoGen) SetContainerStatement(stmt *parser.ContainerStmt) {
	fields := stmt.Fields
	for _, field := range fields {
		g.resolveCustomTypes(field.Type)
		g.adjustExternalStructureToImports(field.Type)
	}

	g.containerStmt = GoContainerStmt{
		PublicName:  utils.ToUpper(stmt.Name),
		PrivateName: utils.ToLower(stmt.Name),

		Fields:      fields,
		DefaultName: stmt.Name,
		ReservedIDs: stmt.ReservedIDs,
		Comment:     stmt.Comment,
	}
}

func (g *GoGen) AddEnumDecls(enumDecls []string) {
	g.enumDecls = append(g.enumDecls, enumDecls...)
}

func (g *GoGen) AddContainerDecls(containerDecls []string) {
	g.containerDecls = append(g.containerDecls, containerDecls...)
}

func (g *GoGen) ProcessImport(stmt *parser.UseStmt, importDirs []string) ([]string, []string) {
	var content []byte
	var err error

	importDirs = append(importDirs, "./")

	for _, importDir := range importDirs {
		trimmedDir := strings.TrimSuffix(importDir, "/")
		fullPath := filepath.Join(trimmedDir, stmt.Path)

		if _, err = os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		content, err = os.ReadFile(fullPath)
		if err == nil {
			break
		}
	}

	if err != nil {
		LogErrorAndExit(g, fmt.Sprintf("Failed to read file: %s. Check again whether your import dirs are correct.", stmt.Path))
	}

	importParser := parser.NewParser(strings.NewReader(string(content)), string(content))
	importNodes := importParser.Parse()

	var goPackage string
	var definePackage string

	var enumDecls = []string{}
	var containerDecls = []string{}

	for _, node := range importNodes {
		switch n := node.(type) {
		case *parser.VarStmt:
			if n.Name == "go_package" {
				goPackage = n.Value
			}
		case *parser.DefineStmt:
			definePackage = n.Package
		case *parser.EnumStmt:
			enumDecls = append(enumDecls, n.Name)
		case *parser.ContainerStmt:
			containerDecls = append(containerDecls, n.Name)
		}
	}

	if goPackage == "" {
		LogErrorAndExit(g, fmt.Sprintf("No 'go_package' variable has been set in imported file '%s'.", stmt.Path))
	}

	splitPackage := strings.Split(goPackage, "/")
	if len(splitPackage) <= 1 {
		LogErrorAndExit(g, fmt.Sprintf("Invalid 'go_package' variable has been set in imported file '%s'.", stmt.Path))
	}

	packageAlias := splitPackage[len(splitPackage)-1]

	importedEnums := make(map[string]string)
	importedContainers := make(map[string]string)

	for _, enum := range enumDecls {
		importedEnums[definePackage+"."+enum] = packageAlias + "." + enum
	}
	for _, container := range containerDecls {
		importedContainers[definePackage+"."+container] = packageAlias + "." + container
	}

	g.enumDecls = append(g.enumDecls, MapValues(importedEnums)...)
	g.containerDecls = append(g.containerDecls, MapValues(importedContainers)...)

	if !slices.Contains(g.importedPackages, goPackage) {
		g.importedPackages = append(g.importedPackages, goPackage)
	}

	MapCopy(g.importedEnumsOrContainers, importedEnums)
	MapCopy(g.importedEnumsOrContainers, importedContainers)

	return MapKeys(importedEnums), MapKeys(importedContainers)
}

func (g *GoGen) joinImportedPackages() string {
	var sb strings.Builder
	iEnd := len(g.importedPackages) - 1
	for i, pkg := range g.importedPackages {
		if i == iEnd {
			sb.WriteString(fmt.Sprintf("	\"%s\"", pkg))
			break
		}
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", pkg))
	}
	return sb.String()
}

func (g *GoGen) GenDefine() string {
	goPackage, found := g.varMap["go_package"]
	if !found {
		LogErrorAndExit(g, "No 'go_package' variable has been set.")
	}

	splitPackage := strings.Split(goPackage, "/")
	if len(splitPackage) <= 1 {
		LogErrorAndExit(g, "Invalid 'go_package' variable has been set.")
	}

	packageAlias := splitPackage[len(splitPackage)-1]

	return fmt.Sprintf(
		`package %s

import (
    "github.com/alvarolm/benc/std"
    "github.com/alvarolm/benc/impl/gen"

%s
)

`, packageAlias, g.joinImportedPackages())
}

func joinUint16(ids []uint16) string {
	var sb strings.Builder
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d", id))
	}
	return sb.String()
}

func (g *GoGen) GenReservedIds() string {
	ctr := g.containerStmt
	return fmt.Sprintf("// Reserved Ids - %s\nvar %sRIds = []uint16{%s}\n\n",
		ctr.DefaultName, ctr.PrivateName, joinUint16(ctr.ReservedIDs))
}

func (g *GoGen) GenStruct() string {
	var sb strings.Builder
	ctr := g.containerStmt

	sb.WriteString(docComment(ctr.Comment, ""))
	sb.WriteString(fmt.Sprintf("// Struct - %s\ntype %s struct {\n",
		ctr.DefaultName, ctr.PublicName))

	g.ForEachCtrFields(func(i int) {
		field := g.field
		sb.WriteString(docComment(field.Comment, "    "))
		sb.WriteString(fmt.Sprintf("    %s %s\n",
			field.PublicName, utils.BencTypeToGolang(field.Type)))
	})

	sb.WriteString("}\n\n")
	return sb.String()
}

func (g *GoGen) GenEnum() string {
	var sb strings.Builder
	enum := g.enumStmt

	sb.WriteString(docComment(enum.Comment, ""))
	sb.WriteString(fmt.Sprintf("// Enum - %s\ntype %s int\nconst (\n",
		enum.DefaultName, enum.PublicName))

	g.ForEachEnumValues(func(i int, value string) {
		if i < len(enum.ValueComments) {
			sb.WriteString(docComment(enum.ValueComments[i], "    "))
		}
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    %s%s %s = iota\n",
				enum.PublicName, value, enum.PublicName))
			return
		}
		sb.WriteString(fmt.Sprintf("    %s%s\n",
			enum.PublicName, value))
	})

	sb.WriteString(")\n\n")
	return sb.String()
}

// GenCustom emits the Go type definition and the three local cast helpers for an
// alias custom type (Form A: `custom Name = string;`). External-codec customs
// (Form B) emit nothing here — they reference a user-provided package.
func (g *GoGen) GenCustom() string {
	stmt := g.customStmt
	if stmt == nil || !stmt.IsAlias {
		return ""
	}

	public := utils.ToUpper(stmt.Name)
	priv := utils.ToLower(stmt.Name)
	base := stmt.BaseType
	goBase := base.Golang()
	suffix := base.String()

	// `bytes` has no plain Unmarshal; default to the cropped variant (matching the
	// primitive default in AppendReturnCopyIfPresent).
	unmarshalSuffix := suffix
	if base == lexer.BYTES {
		unmarshalSuffix = "BytesCropped"
	}

	// Only string/bytes/int/uint take a value argument in their Size function;
	// fixed-width types have a no-argument SizeX().
	sizeTakesArg := base == lexer.STRING || base == lexer.BYTES || base == lexer.INT || base == lexer.UINT

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Custom - %s\ntype %s %s\n\n", stmt.Name, public, goBase))

	if sizeTakesArg {
		sb.WriteString(fmt.Sprintf("func %sSize(v %s) int { return bstd.Size%s(%s(v)) }\n",
			priv, public, suffix, goBase))
	} else {
		sb.WriteString(fmt.Sprintf("func %sSize(_ %s) int { return bstd.Size%s() }\n",
			priv, public, suffix))
	}

	sb.WriteString(fmt.Sprintf("func %sMarshal(n int, b []byte, v %s) int { return bstd.Marshal%s(n, b, %s(v)) }\n",
		priv, public, suffix, goBase))

	sb.WriteString(fmt.Sprintf("func %sUnmarshal(n int, b []byte) (int, %s, error) {\n    n, v, err := bstd.Unmarshal%s(n, b)\n    return n, %s(v), err\n}\n\n",
		priv, public, unmarshalSuffix, public))

	return sb.String()
}

func (g *GoGen) getSizeFunc() string {
	ctr := g.containerStmt
	field := g.field

	switch {
	case field.Type.IsArray:
		acc := fmt.Sprintf("%s.%s", ctr.PrivateName, field.PublicName)
		if field.Type.IsFixedArray() {
			acc += "[:]"
		}
		if field.Type.ChildType.IsCustom || field.Type.ChildType.TokenType == lexer.STRING || field.Type.ChildType.TokenType == lexer.BYTES || field.Type.ChildType.IsAnExternalStructure() || field.Type.ChildType.IsMap || field.Type.ChildType.IsArray {
			return fmt.Sprintf("bstd.SizeSlice(%s, %s)",
				acc, g.getElemSizeFunc(field.Type.ChildType))
		}

		return fmt.Sprintf("bstd.SizeFixedSlice(%s, %s())",
			acc, g.getElemSizeFunc(field.Type.ChildType))
	case field.Type.IsMap:
		return fmt.Sprintf("bstd.SizeMap(%s.%s, %s, %s)",
			ctr.PrivateName, field.PublicName, g.getElemSizeFunc(field.Type.MapKeyType), g.getElemSizeFunc(field.Type.ChildType))
	case field.Type.IsCustom:
		return fmt.Sprintf("%s(%s.%s)",
			field.Type.CustomSizeFn, ctr.PrivateName, field.PublicName)
	case field.Type.IsAnExternalStructure():
		if g.IsEnum(field.Type.ExternalStructure) {
			return fmt.Sprintf("bgenimpl.SizeEnum(%s.%s)",
				ctr.PrivateName, field.PublicName)
		}

		if g.plainGen {
			return fmt.Sprintf("%s.%s.SizePlain()",
				ctr.PrivateName, field.PublicName)
		}
		return fmt.Sprintf("%s.%s.NestedSize(%d)",
			ctr.PrivateName, field.PublicName, field.ID)
	default:
		switch field.Type.TokenType {
		case lexer.STRING, lexer.BYTES, lexer.INT, lexer.UINT:
			return fmt.Sprintf("bstd.Size%s(%s.%s)",
				field.Type.TokenType.String(), ctr.PrivateName, field.PublicName)
		}

		return fmt.Sprintf("bstd.Size%s()",
			field.Type.TokenType.String())
	}
}

// isFixedWidthNumeric reports whether a slice/array element of type t can be marshalled
// with the bulk bstd.Marshal<Type>Slice helpers (which inline the per-element write).
// These are the multi-byte fixed-width numeric types; their bstd.Size is a no-arg constant
// and bstd exposes a matching Marshal<Type.String()>Slice helper.
func isFixedWidthNumeric(t lexer.Token) bool {
	switch t {
	case lexer.INT16, lexer.INT32, lexer.INT64,
		lexer.UINT16, lexer.UINT32, lexer.UINT64,
		lexer.FLOAT32, lexer.FLOAT64:
		return true
	}
	return false
}

func makeExternalStructureUpperOrNot(externalStructure string) string {
	if strings.Contains(externalStructure, ".") {
		return externalStructure
	}
	return utils.ToUpper(externalStructure)
}

func (g *GoGen) getElemSizeFunc(t *parser.Type) string {
	switch {
	case t.IsArray:
		elem := "s"
		if t.IsFixedArray() {
			elem = "s[:]"
		}
		if t.ChildType.IsCustom || t.ChildType.TokenType == lexer.STRING || t.ChildType.TokenType == lexer.BYTES || t.ChildType.IsAnExternalStructure() || t.ChildType.IsMap || t.ChildType.IsArray {
			return fmt.Sprintf("func (s %s) int { return bstd.SizeSlice(%s, %s) }",
				utils.BencTypeToGolang(t), elem, g.getElemSizeFunc(t.ChildType))
		}

		return fmt.Sprintf("func (s %s) int { return bstd.SizeFixedSlice(%s, %s()) }",
			utils.BencTypeToGolang(t), elem, g.getElemSizeFunc(t.ChildType))
	case t.IsMap:
		return fmt.Sprintf("func (s %s) int { return bstd.SizeMap(s, %s, %s) }",
			utils.BencTypeToGolang(t), g.getElemSizeFunc(t.MapKeyType), g.getElemSizeFunc(t.ChildType))
	case t.IsCustom:
		return t.CustomSizeFn
	case t.IsAnExternalStructure():
		if g.IsEnum(t.ExternalStructure) {
			return "bgenimpl.SizeEnum"
		}

		return fmt.Sprintf("func (s %s) int { return s.SizePlain() }",
			makeExternalStructureUpperOrNot(t.ExternalStructure))
	default:
		return "bstd.Size" + t.TokenType.String()
	}
}

func (g *GoGen) GenSize() string {
	var sb strings.Builder
	ctr := g.containerStmt

	sb.WriteString(fmt.Sprintf("// Size - %s\nfunc (%s *%s) Size() int {\n    return %s.NestedSize(0)\n}\n\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName, ctr.PrivateName))

	sb.WriteString(fmt.Sprintf("// Nested Size - %s\nfunc (%s *%s) NestedSize(id uint16) (s int) {\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		field := g.field

		tagSize := 2
		if g.field.ID > 255 {
			tagSize = 3
		}

		sb.WriteString(fmt.Sprintf("    s += %s", g.getSizeFunc()))

		if !g.IsContainer(field.Type.ExternalStructure) {
			sb.WriteString(fmt.Sprintf(" + %d\n", tagSize))
		} else {
			sb.WriteString("\n")
		}
	})

	sb.WriteString("\n    if id > 255 {\n        s += 5\n        return\n    }\n    s += 4\n    return\n}\n\n")
	return sb.String()
}

func (g *GoGen) GenSizePlain() string {
	var sb strings.Builder
	ctr := g.containerStmt

	g.plainGen = true
	defer func() { g.plainGen = false }()

	sb.WriteString(fmt.Sprintf("// SizePlain - %s\nfunc (%s *%s) SizePlain() (s int) {\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		sb.WriteString(fmt.Sprintf("    s += %s\n", g.getSizeFunc()))
	})

	sb.WriteString("    return\n}\n\n")
	return sb.String()
}

func (g *GoGen) getMarshalFunc() string {
	ctr := g.containerStmt
	field := g.field

	switch {
	case field.Type.IsArray:
		if field.Type.IsFixedArray() && field.Type.ChildType.TokenType == lexer.BYTE {
			return fmt.Sprintf("bstd.MarshalFixedByteArray(n, b, %s.%s[:])",
				ctr.PrivateName, field.PublicName)
		}
		acc := fmt.Sprintf("%s.%s", ctr.PrivateName, field.PublicName)
		if field.Type.IsFixedArray() {
			acc += "[:]"
		}
		if isFixedWidthNumeric(field.Type.ChildType.TokenType) {
			return fmt.Sprintf("bstd.Marshal%sSlice(n, b, %s)",
				field.Type.ChildType.TokenType.String(), acc)
		}
		return fmt.Sprintf("bstd.MarshalSlice(n, b, %s, %s)",
			acc, g.getElemMarshalFunc(field.Type.ChildType))
	case field.Type.IsMap:
		return fmt.Sprintf("bstd.MarshalMap(n, b, %s.%s, %s, %s)",
			ctr.PrivateName, field.PublicName, g.getElemMarshalFunc(field.Type.MapKeyType), g.getElemMarshalFunc(field.Type.ChildType))
	case field.Type.IsCustom:
		return fmt.Sprintf("%s(n, b, %s.%s)",
			field.Type.CustomMarshalFn, ctr.PrivateName, field.PublicName)
	case field.Type.IsAnExternalStructure():
		if g.IsEnum(field.Type.ExternalStructure) {
			return fmt.Sprintf("bgenimpl.MarshalEnum(n, b, %s.%s)",
				ctr.PrivateName, field.PublicName)
		}

		if g.plainGen {
			return fmt.Sprintf("%s.%s.MarshalPlain(n, b)",
				ctr.PrivateName, field.PublicName)
		}
		return fmt.Sprintf("%s.%s.NestedMarshal(n, b, %d)",
			ctr.PrivateName, field.PublicName, field.ID)
	default:
		return fmt.Sprintf("bstd.Marshal%s%s(n, b, %s.%s)",
			field.AppendUnsafeIfPresent(), field.Type.TokenType.String(), ctr.PrivateName, field.PublicName)
	}
}

func (g *GoGen) getElemMarshalFunc(t *parser.Type) string {
	switch {
	case t.IsArray:
		if t.IsFixedArray() && t.ChildType.TokenType == lexer.BYTE {
			return fmt.Sprintf("func (n int, b []byte, s %s) int { return bstd.MarshalFixedByteArray(n, b, s[:]) }",
				utils.BencTypeToGolang(t))
		}
		elem := "s"
		if t.IsFixedArray() {
			elem = "s[:]"
		}
		if isFixedWidthNumeric(t.ChildType.TokenType) {
			return fmt.Sprintf("func (n int, b []byte, s %s) int { return bstd.Marshal%sSlice(n, b, %s) }",
				utils.BencTypeToGolang(t), t.ChildType.TokenType.String(), elem)
		}
		return fmt.Sprintf("func (n int, b []byte, s %s) int { return bstd.MarshalSlice(n, b, %s, %s) }",
			utils.BencTypeToGolang(t), elem, g.getElemMarshalFunc(t.ChildType))
	case t.IsMap:
		return fmt.Sprintf("func (n int, b []byte, s %s) int { return bstd.MarshalMap(n, b, s, %s, %s) }",
			utils.BencTypeToGolang(t), g.getElemMarshalFunc(t.MapKeyType), g.getElemMarshalFunc(t.ChildType))
	case t.IsCustom:
		return t.CustomMarshalFn
	case t.IsAnExternalStructure():
		if g.IsEnum(t.ExternalStructure) {
			return "bgenimpl.MarshalEnum"
		}

		return fmt.Sprintf("func (n int, b []byte, s %s) int { return s.MarshalPlain(n, b) }",
			makeExternalStructureUpperOrNot(t.ExternalStructure))
	default:
		return "bstd.Marshal" + t.AppendUnsafeIfPresent() + t.TokenType.String()
	}
}

func (g *GoGen) GenMarshal() string {
	var sb strings.Builder
	ctr := g.containerStmt

	sb.WriteString(fmt.Sprintf("// Marshal - %s\nfunc (%s *%s) Marshal(b []byte) {\n    %s.NestedMarshal(0, b, 0)\n}\n\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName, ctr.PrivateName))

	sb.WriteString(fmt.Sprintf("// Nested Marshal - %s\nfunc (%s *%s) NestedMarshal(tn int, b []byte, id uint16) (n int) {\n    n = bgenimpl.MarshalTag(tn, b, bgenimpl.Container, id)\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		field := g.field

		if !g.IsContainer(field.Type.ExternalStructure) {
			wireTag := g.mapTokenTypeToBgenimplType(field.Type.TokenType)
			if field.Type.IsCustom {
				wireTag = field.Type.CustomWireTag
			}
			sb.WriteString(fmt.Sprintf("    n = bgenimpl.MarshalTag(n, b, bgenimpl.%s, %d)\n",
				wireTag, field.ID))
		}
		sb.WriteString(fmt.Sprintf("    n = %s\n", g.getMarshalFunc()))
	})

	sb.WriteString("\n    n += 2\n    b[n-2] = 1\n    b[n-1] = 1\n    return\n}\n\n")
	return sb.String()
}

func (g *GoGen) GenMarshalPlain() string {
	var sb strings.Builder
	ctr := g.containerStmt

	g.plainGen = true
	defer func() { g.plainGen = false }()

	sb.WriteString(fmt.Sprintf("// MarshalPlain - %s\nfunc (%s *%s) MarshalPlain(tn int, b []byte) (n int) {\n    n = tn\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		sb.WriteString(fmt.Sprintf("    n = %s\n", g.getMarshalFunc()))
	})

	sb.WriteString("    return n\n}\n\n")
	return sb.String()
}

func (g *GoGen) mapTokenTypeToBgenimplType(t lexer.Token) string {
	switch t {
	case lexer.INT, lexer.UINT:
		return "Varint"
	case lexer.STRING, lexer.BYTES:
		return "Bytes"
	case lexer.BOOL, lexer.BYTE:
		return "Fixed8"
	case lexer.INT16, lexer.UINT16:
		return "Fixed16"
	case lexer.INT32, lexer.UINT32, lexer.FLOAT32:
		return "Fixed32"
	case lexer.INT64, lexer.UINT64, lexer.FLOAT64:
		return "Fixed64"
	default:
		return "ArrayMap"
	}
}

func (g *GoGen) getUnmarshalFunc() string {
	ctr := g.containerStmt
	field := g.field

	switch {
	case field.Type.IsArray:
		if field.Type.IsFixedArray() {
			if field.Type.ChildType.TokenType == lexer.BYTE {
				return fmt.Sprintf("bstd.UnmarshalFixedByteArray(n, b, %s.%s[:])",
					ctr.PrivateName, field.PublicName)
			}
			return fmt.Sprintf("bstd.UnmarshalFixedArray(n, b, %s.%s[:], %s)",
				ctr.PrivateName, field.PublicName, g.getElemUnmarshalFunc(field.Type.ChildType))
		}
		return fmt.Sprintf("bstd.UnmarshalSlice[%s](n, b, %s)",
			utils.BencTypeToGolang(field.Type.ChildType), g.getElemUnmarshalFunc(field.Type.ChildType))
	case field.Type.IsMap:
		return fmt.Sprintf("bstd.UnmarshalMap[%s, %s](n, b, %s, %s)",
			utils.BencTypeToGolang(field.Type.MapKeyType), utils.BencTypeToGolang(field.Type.ChildType), g.getElemUnmarshalFunc(field.Type.MapKeyType), g.getElemUnmarshalFunc(field.Type.ChildType))
	case field.Type.IsCustom:
		return fmt.Sprintf("%s(n, b)", field.Type.CustomUnmarshalFn)
	case field.Type.IsAnExternalStructure():
		if g.IsEnum(field.Type.ExternalStructure) {
			return fmt.Sprintf("bgenimpl.UnmarshalEnum[%s](n, b)", field.Type.ExternalStructure)
		}
		if g.plainGen {
			return fmt.Sprintf("%s.%s.UnmarshalPlain(n, b)", ctr.PrivateName, field.PublicName)
		}
		return fmt.Sprintf("bstd.Unmarshal%s%s%s(n, b)", field.AppendUnsafeIfPresent(), field.Type.TokenType.String(), field.AppendReturnCopyIfPresent())
	default:
		return fmt.Sprintf("bstd.Unmarshal%s%s%s(n, b)", field.AppendUnsafeIfPresent(), field.Type.TokenType.String(), field.AppendReturnCopyIfPresent())
	}
}

func (g *GoGen) getElemUnmarshalFunc(t *parser.Type) string {
	switch {
	case t.IsArray:
		if t.IsFixedArray() {
			gt := utils.BencTypeToGolang(t)
			if t.ChildType.TokenType == lexer.BYTE {
				return fmt.Sprintf("func (n int, b []byte) (int, %s, error) { var a %s; n, err := bstd.UnmarshalFixedByteArray(n, b, a[:]); return n, a, err }",
					gt, gt)
			}
			return fmt.Sprintf("func (n int, b []byte) (int, %s, error) { var a %s; n, err := bstd.UnmarshalFixedArray(n, b, a[:], %s); return n, a, err }",
				gt, gt, g.getElemUnmarshalFunc(t.ChildType))
		}
		return fmt.Sprintf("func (n int, b []byte) (int, %s, error) { return bstd.UnmarshalSlice[%s](n, b, %s) }",
			utils.BencTypeToGolang(t), utils.BencTypeToGolang(t.ChildType), g.getElemUnmarshalFunc(t.ChildType))
	case t.IsMap:
		return fmt.Sprintf("func (n int, b []byte) (int, %s, error) { return bstd.UnmarshalMap[%s, %s](n, b, %s, %s) }",
			utils.BencTypeToGolang(t), utils.BencTypeToGolang(t.MapKeyType), utils.BencTypeToGolang(t.ChildType), g.getElemUnmarshalFunc(t.MapKeyType), g.getElemUnmarshalFunc(t.ChildType))
	case t.IsCustom:
		return t.CustomUnmarshalFn
	case t.IsAnExternalStructure():
		if g.IsEnum(t.ExternalStructure) {
			return "bgenimpl.UnmarshalEnum"
		}
		return fmt.Sprintf("func (n int, b []byte, s *%s) (int, error) { return s.UnmarshalPlain(n, b) }",
			makeExternalStructureUpperOrNot(t.ExternalStructure))
	default:
		return "bstd.Unmarshal" + t.AppendUnsafeIfPresent() + t.TokenType.String() + t.AppendReturnCopyIfPresent()
	}
}

func (g *GoGen) GenUnmarshal() string {
	var sb strings.Builder
	ctr := g.containerStmt

	sb.WriteString(fmt.Sprintf("// Unmarshal - %s\nfunc (%s *%s) Unmarshal(b []byte) (err error) {\n    _, err = %s.NestedUnmarshal(0, b, []uint16{}, 0)\n    return\n}\n\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName, ctr.PrivateName))

	sb.WriteString(fmt.Sprintf("// Nested Unmarshal - %s\nfunc (%s *%s) NestedUnmarshal(tn int, b []byte, r []uint16, id uint16) (n int, err error) {\n    var ok bool\n    if n, ok, err = bgenimpl.HandleCompatibility(tn, b, r, id); !ok {\n        if err == bgenimpl.ErrEof {\n            return n, nil\n        }\n        return\n    }\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		field := g.field

		if g.IsContainer(field.Type.ExternalStructure) {
			sb.WriteString(fmt.Sprintf("    if n, err = %s.%s.NestedUnmarshal(n, b, %sRIds, %d); err != nil {\n        return\n    }\n",
				ctr.PrivateName, field.PublicName, ctr.PrivateName, field.ID))
			return
		}

		sb.WriteString(fmt.Sprintf("    if n, ok, err = bgenimpl.HandleCompatibility(n, b, %sRIds, %d); err != nil {\n        if err == bgenimpl.ErrEof {\n            return n, nil\n        }\n        return\n    }\n",
			ctr.PrivateName, field.ID))

		// Fixed arrays decode in place (UnmarshalFixedArray returns (int, error)),
		// so they drop the value target — but unlike containers they still carry a
		// per-field tag, so the HandleCompatibility block above is kept.
		if field.Type.IsFixedArray() {
			sb.WriteString(fmt.Sprintf("    if ok {\n        if n, err = %s; err != nil {\n            return\n        }\n    }\n",
				g.getUnmarshalFunc()))
			return
		}

		sb.WriteString(fmt.Sprintf("    if ok {\n        if n, %s.%s, err = %s; err != nil {\n            return\n        }\n    }\n",
			ctr.PrivateName, field.PublicName, g.getUnmarshalFunc()))
	})

	sb.WriteString("    n += 2\n    return\n}\n\n")
	return sb.String()
}

func (g *GoGen) GenUnmarshalPlain() string {
	var sb strings.Builder
	ctr := g.containerStmt

	g.plainGen = true
	defer func() { g.plainGen = false }()

	sb.WriteString(fmt.Sprintf("// UnmarshalPlain - %s\nfunc (%s *%s) UnmarshalPlain(tn int, b []byte) (n int, err error) {\n    n = tn\n",
		ctr.DefaultName, ctr.PrivateName, ctr.PublicName))

	g.ForEachCtrFields(func(_ int) {
		field := g.field

		if g.IsContainer(field.Type.ExternalStructure) || field.Type.IsFixedArray() {
			sb.WriteString(fmt.Sprintf("    if n, err = %s; err != nil {\n        return\n    }\n",
				g.getUnmarshalFunc()))
			return
		}

		sb.WriteString(fmt.Sprintf("    if n, %s.%s, err = %s; err != nil {\n        return\n    }\n",
			ctr.PrivateName, field.PublicName, g.getUnmarshalFunc()))
	})

	sb.WriteString("    return\n}\n\n")
	return sb.String()
}

// taken from exp/maps

// Values returns the values of the map m.
// The values will be in an indeterminate order.
//
// The simplest true equivalent using the standard library is:
//
//	slices.AppendSeq(make([]V, 0, len(m)), MapValues(m))
func MapValues[M ~map[K]V, K comparable, V any](m M) []V {

	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

// Copy copies all key/value pairs in src adding them to dst.
// When a key in src is already present in dst,
// the value in dst will be overwritten by the value associated
// with the key in src.
//
//go:fix inline
func MapCopy[M1 ~map[K]V, M2 ~map[K]V, K comparable, V any](dst M1, src M2) {
	MapCopy(dst, src)
}

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
//
// The simplest true equivalent using the standard library  is:
//
//	slices.AppendSeq(make([]K, 0, len(m)), maps.Keys(m))
func MapKeys[M ~map[K]V, K comparable, V any](m M) []K {

	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}
