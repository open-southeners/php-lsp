package models

import (
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/open-southeners/php-lsp/internal/parser"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

const eloquentModelFQN = "Illuminate\\Database\\Eloquent\\Model"

// Eloquent relation types grouped by cardinality.
var singularRelations = map[string]bool{
	"HasOne": true, "BelongsTo": true, "MorphOne": true, "MorphTo": true,
	"HasOneThrough": true,
}

var pluralRelations = map[string]bool{
	"HasMany": true, "BelongsToMany": true, "MorphMany": true,
	"MorphToMany": true, "MorphedByMany": true, "HasManyThrough": true,
}

// allRelationTypes is the union of singular and plural for quick lookup.
var allRelationTypes map[string]bool

func init() {
	allRelationTypes = make(map[string]bool, len(singularRelations)+len(pluralRelations))
	for k := range singularRelations {
		allRelationTypes[k] = true
	}
	for k := range pluralRelations {
		allRelationTypes[k] = true
	}
}

// Regex to extract $this->hasMany(Post::class) style calls.
var relationCallRe = regexp.MustCompile(
	`\$this\s*->\s*(hasOne|hasMany|belongsTo|belongsToMany|morphOne|morphMany|morphTo|morphToMany|morphedByMany|hasOneThrough|hasManyThrough)\s*\(\s*([A-Za-z_\\]+)::class`,
)

// Regex for legacy accessor: getNameAttribute()
var legacyAccessorRe = regexp.MustCompile(`^get([A-Z][A-Za-z0-9]*)Attribute$`)

// AnalyzeEloquentModels scans all classes extending Eloquent Model and injects
// virtual properties for relations and accessors/mutators.
func AnalyzeEloquentModels(index *symbols.Index, rootPath string) {
	models := index.GetDescendants(eloquentModelFQN)
	for _, model := range models {
		analyzeModel(index, model)
	}
}

func analyzeModel(index *symbols.Index, model *symbols.Symbol) {
	// Read the source file to extract method bodies
	path := symbols.URIToPath(model.URI)
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(content)
	lines := strings.Split(source, "\n")

	file := parser.ParseFile(source)
	if file == nil {
		return
	}

	// Find the class in the parsed file
	var classNode *parser.ClassNode
	for i := range file.Classes {
		fqn := file.Namespace + "\\" + file.Classes[i].Name
		if file.Namespace == "" {
			fqn = file.Classes[i].Name
		}
		if fqn == model.FQN {
			classNode = &file.Classes[i]
			break
		}
	}
	if classNode == nil {
		return
	}

	resolve := func(name string) string {
		return resolveWithUses(name, file.Namespace, file.Uses)
	}

	for _, method := range classNode.Methods {
		// Check for relation return type
		returnShort := shortClassName(method.ReturnType.Name)
		if allRelationTypes[returnShort] {
			body := extractMethodBody(lines, method.StartLine, method.EndLine)
			injectRelation(index, model, method.Name, returnShort, body, resolve)
			continue
		}

		// Check for relation calls in body (no explicit return type)
		if method.ReturnType.Name == "" || method.ReturnType.Name == "mixed" {
			body := extractMethodBody(lines, method.StartLine, method.EndLine)
			if match := relationCallRe.FindStringSubmatch(body); match != nil {
				relType := match[1]
				relShort := ucFirst(relType)
				injectRelation(index, model, method.Name, relShort, body, resolve)
				continue
			}
		}

		// Check for legacy accessor: getNameAttribute() → virtual $name
		if m := legacyAccessorRe.FindStringSubmatch(method.Name); m != nil {
			propName := "$" + snakeCase(m[1])
			retType := method.ReturnType.Name
			if retType == "" && method.DocComment != "" {
				if doc := parser.ParseDocBlock(method.DocComment); doc != nil && doc.Return.Type != "" {
					retType = doc.Return.Type
				}
			}
			index.AddVirtualMember(model.FQN, &symbols.Symbol{
				Name:       propName,
				FQN:        model.FQN + "::" + propName,
				Kind:       symbols.KindProperty,
				URI:        model.URI,
				Visibility: "public",
				Type:       resolve(retType),
				IsVirtual:  true,
				DocComment: "Accessor from " + method.Name + "()",
			})
			continue
		}

		// Check for modern accessor: protected function name(): Attribute
		resolvedReturn := resolve(method.ReturnType.Name)
		if resolvedReturn == "Illuminate\\Database\\Eloquent\\Casts\\Attribute" {
			propName := "$" + method.Name
			// Try to infer type from docblock @return
			retType := ""
			if method.DocComment != "" {
				if doc := parser.ParseDocBlock(method.DocComment); doc != nil && doc.Return.Type != "" {
					retType = doc.Return.Type
				}
			}
			if retType == "" {
				retType = "mixed"
			}
			index.AddVirtualMember(model.FQN, &symbols.Symbol{
				Name:       propName,
				FQN:        model.FQN + "::" + propName,
				Kind:       symbols.KindProperty,
				URI:        model.URI,
				Visibility: "public",
				Type:       resolve(retType),
				IsVirtual:  true,
				DocComment: "Accessor from " + method.Name + "()",
			})
		}
	}
}

// injectRelation creates virtual property and ensures the method also has the right return type.
func injectRelation(index *symbols.Index, model *symbols.Symbol, methodName, relType, body string, resolve func(string) string) {
	// Extract related model from body: $this->hasMany(Post::class)
	relatedModel := ""
	if match := relationCallRe.FindStringSubmatch(body); match != nil {
		relatedModel = resolve(match[2])
	}

	// Determine property type based on relation cardinality
	propType := "mixed"
	if relatedModel != "" {
		if singularRelations[relType] {
			propType = "?" + relatedModel
		} else {
			propType = "Illuminate\\Database\\Eloquent\\Collection"
		}
	}

	// Create virtual property (e.g. $user->posts as Collection)
	propName := "$" + methodName
	index.AddVirtualMember(model.FQN, &symbols.Symbol{
		Name:       propName,
		FQN:        model.FQN + "::" + propName,
		Kind:       symbols.KindProperty,
		URI:        model.URI,
		Visibility: "public",
		Type:       propType,
		IsVirtual:  true,
		DocComment: relType + " relation",
	})
}

// extractMethodBody returns the source text between startLine and endLine (0-indexed lines array).
func extractMethodBody(lines []string, startLine, endLine int) string {
	// Parser lines are 0-indexed
	if startLine < 0 || endLine < startLine {
		return ""
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}
	return strings.Join(lines[startLine:endLine+1], "\n")
}

// resolveWithUses resolves a type name using the file's use statements.
func resolveWithUses(name, ns string, uses []parser.UseNode) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "?") {
		return "?" + resolveWithUses(name[1:], ns, uses)
	}
	if symbols.IsPHPBuiltinType(strings.ToLower(name)) {
		return name
	}
	if strings.HasPrefix(name, "\\") {
		return strings.TrimPrefix(name, "\\")
	}
	parts := strings.SplitN(name, "\\", 2)
	for _, u := range uses {
		if u.Alias == parts[0] {
			if len(parts) > 1 {
				return u.FullName + "\\" + parts[1]
			}
			return u.FullName
		}
	}
	if ns != "" {
		return ns + "\\" + name
	}
	return name
}

// shortClassName extracts the short class name from a potentially qualified name.
func shortClassName(name string) string {
	if i := strings.LastIndex(name, "\\"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// snakeCase converts PascalCase to snake_case.
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
