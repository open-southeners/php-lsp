package hover

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/open-southeners/php-lsp/internal/container"
	"github.com/open-southeners/php-lsp/internal/parser"
	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

type Provider struct {
	index     *symbols.Index
	container *container.ContainerAnalyzer
	framework string
}

func NewProvider(index *symbols.Index, ca *container.ContainerAnalyzer, framework string) *Provider {
	return &Provider{index: index, container: ca, framework: framework}
}

func (p *Provider) GetHover(uri, source string, pos protocol.Position) *protocol.Hover {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return nil
	}
	line := lines[pos.Line]
	word := getWordAt(source, pos)
	if word == "" {
		return nil
	}

	file := parser.ParseFile(source)

	// Handle $variable hover
	if strings.HasPrefix(word, "$") {
		return p.hoverVariable(file, source, pos, word)
	}

	// Handle self/static/parent keywords — resolve to enclosing class
	if word == "self" || word == "static" || word == "parent" {
		if file != nil {
			var classFQN string
			if word == "parent" {
				enclosing := p.findEnclosingClass(file, pos)
				if enclosing != "" {
					chain := p.index.GetInheritanceChain(enclosing)
					if len(chain) > 0 {
						classFQN = chain[0]
					}
				}
			} else {
				classFQN = p.findEnclosingClass(file, pos)
			}
			if classFQN != "" {
				if sym := p.index.Lookup(classFQN); sym != nil {
					content := p.formatHover(sym)
					if content != "" {
						return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
					}
				}
			}
		}
	}

	// Find the start position of the word on the line
	wordStart := pos.Character
	for wordStart > 0 && isWordChar(line[wordStart-1]) {
		wordStart--
	}

	// Check for -> or :: access context
	if classFQN := p.resolveAccessChain(line, wordStart, file, source, pos); classFQN != "" {
		if sym := p.findMember(classFQN, word); sym != nil {
			content := p.formatHover(sym)
			if content != "" {
				return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
			}
		}
	}

	// Resolve the word via use statements
	if file != nil {
		for _, u := range file.Uses {
			if u.Alias == word {
				if sym := p.index.Lookup(u.FullName); sym != nil {
					content := p.formatHover(sym)
					if content != "" {
						return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
					}
				}
			}
		}
		// Try resolving as a class name in the current namespace context
		fqn := p.resolveClassName(word, file)
		if fqn != word {
			if sym := p.index.Lookup(fqn); sym != nil {
				content := p.formatHover(sym)
				if content != "" {
					return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
				}
			}
		}
	}

	// If the word contains backslashes (FQN like Monolog\Logger), try direct FQN lookup
	if strings.Contains(word, "\\") {
		if sym := p.index.Lookup(word); sym != nil {
			content := p.formatHover(sym)
			if content != "" {
				return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
			}
		}
	}

	// Fallback: lookup by short name
	lookupName := word
	if idx := strings.LastIndex(word, "\\"); idx >= 0 {
		lookupName = word[idx+1:]
	}
	syms := p.index.LookupByName(lookupName)
	if len(syms) == 0 {
		return nil
	}
	content := p.formatHover(syms[0])
	if content == "" {
		return nil
	}
	return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: content}}
}

// resolveAccessChain walks left through a chain of -> and :: accesses and
// returns the FQN of the class that owns the member at wordStart.
// E.g. for "$this->logger->info()", if wordStart points at "info",
// it resolves $this -> Service, finds property "logger" -> Logger type, returns Logger FQN.
func (p *Provider) resolveAccessChain(line string, wordStart int, file *parser.FileNode, source string, pos protocol.Position) string {
	i := wordStart

	// Skip whitespace before the word
	for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
		i--
	}
	if i < 2 {
		return ""
	}

	// Check for -> or ::
	var op string
	if line[i-2] == '-' && line[i-1] == '>' {
		op = "->"
		i -= 2
	} else if line[i-2] == ':' && line[i-1] == ':' {
		op = "::"
		i -= 2
	} else {
		return ""
	}
	_ = op

	// Skip whitespace before operator
	for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
		i--
	}

	// Skip past a method call's closing paren: $foo->bar()->baz
	if i > 0 && line[i-1] == ')' {
		depth := 1
		i--
		for i > 0 && depth > 0 {
			i--
			if line[i] == ')' {
				depth++
			} else if line[i] == '(' {
				depth--
			}
		}
		// Now i points at '(', skip whitespace before it
		for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
			i--
		}
	}

	// Extract the target word
	end := i
	for i > 0 && isWordChar(line[i-1]) {
		i--
	}
	// Include $ for variables
	if i > 0 && line[i-1] == '$' {
		i--
	}
	if i >= end {
		return ""
	}
	target := line[i:end]

	if file == nil {
		return ""
	}

	// Resolve the target to a class FQN
	switch target {
	case "$this", "self", "static":
		return p.findEnclosingClass(file, pos)
	case "parent":
		classFQN := p.findEnclosingClass(file, pos)
		if classFQN == "" {
			return ""
		}
		chain := p.index.GetInheritanceChain(classFQN)
		if len(chain) > 0 {
			return chain[0]
		}
		return ""
	}

	if strings.HasPrefix(target, "$") {
		// Variable: resolve its type
		typeFQN := p.resolveVariableType(target, file, source, pos)
		return typeFQN
	}

	// Bare word target: could be a class name (for static access)
	// or a chained property/method (e.g. the "logger" in "$this->logger->info")
	// First, try as a class name
	if fqn := p.resolveClassName(target, file); fqn != "" {
		if p.index.Lookup(fqn) != nil {
			return fqn
		}
	}

	// Otherwise, recursively resolve the chain to get the owner class,
	// then find the target as a member and return its type.
	ownerFQN := p.resolveAccessChain(line, i, file, source, pos)
	if ownerFQN == "" {
		return ""
	}
	member := p.findMember(ownerFQN, target)
	if member == nil {
		return ""
	}
	return p.memberType(member, file)
}

// memberType returns the resolved FQN of the type that a member evaluates to.
func (p *Provider) memberType(member *symbols.Symbol, file *parser.FileNode) string {
	var typeName string
	switch member.Kind {
	case symbols.KindProperty:
		typeName = member.Type
	case symbols.KindMethod:
		typeName = member.ReturnType
	default:
		return ""
	}
	if typeName == "" || typeName == "void" || typeName == "mixed" {
		return ""
	}
	// Handle self/static return types
	if typeName == "self" || typeName == "static" {
		return member.ParentFQN
	}
	return p.resolveClassName(typeName, file)
}

// findEnclosingClass returns the FQN of the class that contains the given position.
func (p *Provider) findEnclosingClass(file *parser.FileNode, pos protocol.Position) string {
	for _, cls := range file.Classes {
		if pos.Line >= cls.StartLine {
			fqn := cls.FullName
			if fqn == "" {
				fqn = buildFQN(file.Namespace, cls.Name)
			}
			return fqn
		}
	}
	return ""
}

// resolveVariableType tries to infer the type of a variable from context.
func (p *Provider) resolveVariableType(varName string, file *parser.FileNode, source string, pos protocol.Position) string {
	// 1. Check method/function parameter type hints in the enclosing scope
	enclosingMethod := p.findEnclosingMethod(file, pos)
	if enclosingMethod != nil {
		for _, param := range enclosingMethod.Params {
			if param.Name == varName {
				return p.resolveClassName(param.Type.Name, file)
			}
		}
	}

	// 2. Check class properties for $this->prop patterns
	// (handled at chain level, but also check promoted constructor params)
	for _, cls := range file.Classes {
		for _, prop := range cls.Properties {
			if "$"+prop.Name == varName && prop.Type.Name != "" {
				return p.resolveClassName(prop.Type.Name, file)
			}
		}
	}

	lines := strings.Split(source, "\n")
	bare := strings.TrimPrefix(varName, "$")

	// 3. Look for `$var = new ClassName(...)` assignments before the hover position
	newPattern := regexp.MustCompile(`\$` + regexp.QuoteMeta(bare) + `\s*=\s*new\s+([A-Za-z_\\]+)`)
	for i := pos.Line; i >= 0 && i >= pos.Line-200; i-- {
		if i >= len(lines) {
			continue
		}
		if m := newPattern.FindStringSubmatch(lines[i]); m != nil {
			return p.resolveClassName(m[1], file)
		}
	}

	// 4. Check @var annotations: /** @var ClassName $var */
	varDocPattern := regexp.MustCompile(`@var\s+([A-Za-z_\\]+)\s+\$` + regexp.QuoteMeta(bare) + `\b`)
	for i := pos.Line; i >= 0 && i >= pos.Line-5; i-- {
		if i >= len(lines) {
			continue
		}
		if m := varDocPattern.FindStringSubmatch(lines[i]); m != nil {
			return p.resolveClassName(m[1], file)
		}
	}

	return ""
}

// findEnclosingMethod returns the method node that contains the given position.
func (p *Provider) findEnclosingMethod(file *parser.FileNode, pos protocol.Position) *parser.MethodNode {
	if file == nil {
		return nil
	}
	for ci := len(file.Classes) - 1; ci >= 0; ci-- {
		cls := file.Classes[ci]
		if pos.Line < cls.StartLine {
			continue
		}
		var best *parser.MethodNode
		for mi := range cls.Methods {
			m := &cls.Methods[mi]
			if pos.Line >= m.StartLine {
				if best == nil || m.StartLine > best.StartLine {
					best = m
				}
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// resolveClassName resolves a short or partially-qualified class name to a FQN
// using use statements and the file's namespace.
func (p *Provider) resolveClassName(name string, file *parser.FileNode) string {
	if name == "" {
		return ""
	}
	if file == nil {
		return name
	}
	// Already fully qualified
	if strings.HasPrefix(name, "\\") {
		return strings.TrimPrefix(name, "\\")
	}
	// Strip nullable
	if strings.HasPrefix(name, "?") {
		name = name[1:]
	}

	parts := strings.SplitN(name, "\\", 2)
	for _, u := range file.Uses {
		if u.Alias == parts[0] {
			if len(parts) > 1 {
				return u.FullName + "\\" + parts[1]
			}
			return u.FullName
		}
	}
	if file.Namespace != "" {
		fqn := file.Namespace + "\\" + name
		if p.index.Lookup(fqn) != nil {
			return fqn
		}
	}
	return name
}

// findMember looks up a member (method, property, constant) on a class,
// traversing the inheritance chain and traits.
func (p *Provider) findMember(classFQN, memberName string) *symbols.Symbol {
	members := p.index.GetClassMembers(classFQN)
	for _, m := range members {
		if m.Name == memberName || m.Name == "$"+memberName {
			return m
		}
	}
	return nil
}

func (p *Provider) hoverVariable(file *parser.FileNode, source string, pos protocol.Position, varName string) *protocol.Hover {
	if file == nil {
		return nil
	}

	// Try to resolve the variable type
	typeName := p.resolveVariableType(varName, file, source, pos)
	if typeName != "" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**variable** `%s`\n\n", varName))
		sb.WriteString(fmt.Sprintf("```php\n%s %s\n```\n", typeName, varName))
		if sym := p.index.Lookup(typeName); sym != nil {
			if sym.DocComment != "" {
				if doc := parser.ParseDocBlock(sym.DocComment); doc != nil && doc.Summary != "" {
					sb.WriteString("\n" + doc.Summary + "\n")
				}
			}
		}
		p.appendContainerBinding(&sb, typeName)
		return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: sb.String()}}
	}

	// Fallback: search all method params in file
	for _, cls := range file.Classes {
		for _, m := range cls.Methods {
			for _, param := range m.Params {
				if param.Name == varName {
					t := param.Type.Name
					if t == "" {
						t = "mixed"
					}
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("**parameter** `%s`\n\n", varName))
					sb.WriteString(fmt.Sprintf("```php\n%s %s\n```\n", t, varName))
					p.appendContainerBinding(&sb, t)
					return &protocol.Hover{Contents: protocol.MarkupContent{Kind: "markdown", Value: sb.String()}}
				}
			}
		}
	}
	return nil
}

func (p *Provider) formatHover(sym *symbols.Symbol) string {
	var sb strings.Builder

	// === Header: kind label + FQN ===
	switch sym.Kind {
	case symbols.KindClass:
		sb.WriteString(fmt.Sprintf("**class** `%s`\n\n", sym.FQN))
		// Declaration with modifiers
		sb.WriteString("```php\n")
		if sym.IsFinal {
			sb.WriteString("final ")
		}
		if sym.IsAbstract {
			sb.WriteString("abstract ")
		}
		if sym.IsReadonly {
			sb.WriteString("readonly ")
		}
		sb.WriteString("class " + sym.Name)
		if sym.Extends != "" {
			sb.WriteString(" extends " + sym.Extends)
		}
		if len(sym.Implements) > 0 {
			sb.WriteString(" implements " + strings.Join(sym.Implements, ", "))
		}
		sb.WriteString("\n```\n")
		// Implemented by (for classes that act as base)
		if impls := p.index.GetImplementors(sym.FQN); len(impls) > 0 {
			sb.WriteString("\n**Implemented by:**\n")
			for _, impl := range impls {
				sb.WriteString(fmt.Sprintf("- `%s`\n", impl.FQN))
			}
		}

	case symbols.KindInterface:
		sb.WriteString(fmt.Sprintf("**interface** `%s`\n\n", sym.FQN))
		sb.WriteString(fmt.Sprintf("```php\ninterface %s\n```\n", sym.Name))
		if impls := p.index.GetImplementors(sym.FQN); len(impls) > 0 {
			sb.WriteString("\n**Implementations:**\n")
			for _, impl := range impls {
				sb.WriteString(fmt.Sprintf("- `%s`\n", impl.FQN))
			}
		}

	case symbols.KindMethod:
		sb.WriteString(fmt.Sprintf("**method** `%s`\n\n", sym.FQN))
		sb.WriteString("```php\n")
		vis := sym.Visibility
		if vis == "" {
			vis = "public"
		}
		if sym.IsAbstract {
			sb.WriteString("abstract ")
		}
		if sym.IsFinal {
			sb.WriteString("final ")
		}
		sb.WriteString(vis)
		if sym.IsStatic {
			sb.WriteString(" static")
		}
		sb.WriteString(fmt.Sprintf(" function %s%s", sym.Name, fmtParams(sym.Params)))
		if sym.ReturnType != "" {
			sb.WriteString(": " + sym.ReturnType)
		}
		sb.WriteString("\n```\n")
		// Override/implements detection
		if sym.ParentFQN != "" {
			p.appendMethodOrigin(&sb, sym)
		}

	case symbols.KindFunction:
		sb.WriteString(fmt.Sprintf("**function** `%s`\n\n", sym.FQN))
		sb.WriteString(fmt.Sprintf("```php\nfunction %s%s", sym.Name, fmtParams(sym.Params)))
		if sym.ReturnType != "" {
			sb.WriteString(": " + sym.ReturnType)
		}
		sb.WriteString("\n```\n")

	case symbols.KindProperty:
		sb.WriteString(fmt.Sprintf("**property** `%s`\n\n", sym.FQN))
		vis := sym.Visibility
		if vis == "" {
			vis = "public"
		}
		t := sym.Type
		if t == "" {
			t = "mixed"
		}
		propName := sym.Name
		if !strings.HasPrefix(propName, "$") {
			propName = "$" + propName
		}
		sb.WriteString(fmt.Sprintf("```php\n%s", vis))
		if sym.IsStatic {
			sb.WriteString(" static")
		}
		sb.WriteString(fmt.Sprintf(" %s %s\n```\n", t, propName))
		if sym.ParentFQN != "" {
			sb.WriteString(fmt.Sprintf("\nDefined in `%s`\n", sym.ParentFQN))
		}

	case symbols.KindEnum:
		sb.WriteString(fmt.Sprintf("**enum** `%s`\n\n", sym.FQN))
		sb.WriteString("```php\nenum " + sym.Name)
		if sym.BackedType != "" {
			sb.WriteString(": " + sym.BackedType)
		}
		if len(sym.Implements) > 0 {
			sb.WriteString(" implements " + strings.Join(sym.Implements, ", "))
		}
		sb.WriteString("\n```\n")

	case symbols.KindEnumCase:
		sb.WriteString(fmt.Sprintf("**case** `%s`\n\n", sym.FQN))
		sb.WriteString("```php\ncase " + sym.Name)
		if sym.Value != "" {
			sb.WriteString(" = " + sym.Value)
		}
		sb.WriteString("\n```\n")
		if sym.ParentFQN != "" {
			sb.WriteString(fmt.Sprintf("\nDefined in `%s`\n", sym.ParentFQN))
		}

	case symbols.KindConstant:
		sb.WriteString(fmt.Sprintf("**constant** `%s`\n\n", sym.FQN))
		sb.WriteString("```php\nconst " + sym.Name)
		if sym.Value != "" {
			sb.WriteString(" = " + sym.Value)
		}
		sb.WriteString("\n```\n")
		if sym.ParentFQN != "" {
			sb.WriteString(fmt.Sprintf("\nDefined in `%s`\n", sym.ParentFQN))
		}

	case symbols.KindTrait:
		sb.WriteString(fmt.Sprintf("**trait** `%s`\n\n", sym.FQN))
		sb.WriteString(fmt.Sprintf("```php\ntrait %s\n```\n", sym.Name))
	}

	// === DocBlock section ===
	doc := p.getEffectiveDocBlock(sym)
	if doc != nil {
		if doc.Summary != "" {
			sb.WriteString("\n" + doc.Summary + "\n")
		}
		if doc.Deprecated {
			msg := doc.DeprecatedMsg
			if msg == "" {
				msg = "This symbol is deprecated."
			}
			sb.WriteString(fmt.Sprintf("\n**⚠ Deprecated:** %s\n", msg))
		}
		if len(doc.Params) > 0 {
			sb.WriteString("\n**Params**\n")
			for _, param := range doc.Params {
				line := "- "
				if param.Name != "" {
					line += "`" + param.Name + "` "
				}
				if param.Type != "" {
					line += "`" + param.Type + "`"
				}
				if param.Description != "" {
					line += " — " + param.Description
				}
				sb.WriteString(line + "\n")
			}
		}
		if doc.Return.Type != "" {
			ret := fmt.Sprintf("\n**Returns** `%s`", doc.Return.Type)
			if doc.Return.Description != "" {
				ret += " — " + doc.Return.Description
			}
			sb.WriteString(ret + "\n")
		}
		if len(doc.Throws) > 0 {
			sb.WriteString("\n**Throws**\n")
			for _, th := range doc.Throws {
				line := "- `" + th.Type + "`"
				if th.Description != "" {
					line += " — " + th.Description
				}
				sb.WriteString(line + "\n")
			}
		}
		// Show @template, @mixin, @see, @property-* from tags
		for _, tagName := range []string{"template", "mixin", "see", "property", "property-read", "property-write", "method"} {
			if vals, ok := doc.Tags[tagName]; ok && len(vals) > 0 {
				label := "@" + tagName
				for _, v := range vals {
					sb.WriteString(fmt.Sprintf("\n`%s %s`\n", label, v))
				}
			}
		}
	}

	// === Container binding (interface/class) ===
	switch sym.Kind {
	case symbols.KindInterface, symbols.KindClass:
		p.appendContainerBinding(&sb, sym.FQN)
	}

	// === PHP Manual link ===
	if url := phpManualURL(sym); url != "" {
		sb.WriteString(fmt.Sprintf("\n[PHP Manual](%s)\n", url))
	}

	return sb.String()
}

// appendContainerBinding adds container binding info if available.
func (p *Provider) appendContainerBinding(sb *strings.Builder, fqn string) {
	if binding := p.container.ResolveDependency(fqn); binding != nil {
		sb.WriteString(fmt.Sprintf("\n---\n**Container Binding**\n- Concrete: `%s`\n- Singleton: %v\n", binding.Concrete, binding.Singleton))
	}
}

// appendMethodOrigin detects if a method overrides a parent method or implements an interface method.
func (p *Provider) appendMethodOrigin(sb *strings.Builder, sym *symbols.Symbol) {
	// Check interfaces
	ifaces := p.index.GetImplementedInterfaces(sym.ParentFQN)
	for _, ifaceFQN := range ifaces {
		ifaceSym := p.index.Lookup(ifaceFQN)
		if ifaceSym == nil {
			continue
		}
		for _, child := range ifaceSym.Children {
			if child.Kind == symbols.KindMethod && child.Name == sym.Name {
				sb.WriteString(fmt.Sprintf("\nImplements `%s::%s`\n", ifaceFQN, sym.Name))
				return
			}
		}
	}
	// Check parent chain
	chain := p.index.GetInheritanceChain(sym.ParentFQN)
	for _, parentFQN := range chain {
		parentSym := p.index.Lookup(parentFQN)
		if parentSym == nil {
			continue
		}
		for _, child := range parentSym.Children {
			if child.Kind == symbols.KindMethod && child.Name == sym.Name {
				sb.WriteString(fmt.Sprintf("\nOverrides `%s::%s`\n", parentFQN, sym.Name))
				return
			}
		}
	}
	// Default: show defined in
	sb.WriteString(fmt.Sprintf("\nDefined in `%s`\n", sym.ParentFQN))
}

// getEffectiveDocBlock returns the docblock for a symbol, falling back to parent/interface docs.
func (p *Provider) getEffectiveDocBlock(sym *symbols.Symbol) *parser.DocBlock {
	if sym.DocComment != "" {
		if doc := parser.ParseDocBlock(sym.DocComment); doc != nil {
			return doc
		}
	}
	// For methods, try inheriting from parent or interface
	if sym.Kind == symbols.KindMethod && sym.ParentFQN != "" {
		// Check interfaces
		ifaces := p.index.GetImplementedInterfaces(sym.ParentFQN)
		for _, ifaceFQN := range ifaces {
			ifaceSym := p.index.Lookup(ifaceFQN)
			if ifaceSym == nil {
				continue
			}
			for _, child := range ifaceSym.Children {
				if child.Kind == symbols.KindMethod && child.Name == sym.Name && child.DocComment != "" {
					if doc := parser.ParseDocBlock(child.DocComment); doc != nil {
						return doc
					}
				}
			}
		}
		// Check parent chain
		chain := p.index.GetInheritanceChain(sym.ParentFQN)
		for _, parentFQN := range chain {
			parentSym := p.index.Lookup(parentFQN)
			if parentSym == nil {
				continue
			}
			for _, child := range parentSym.Children {
				if child.Kind == symbols.KindMethod && child.Name == sym.Name && child.DocComment != "" {
					if doc := parser.ParseDocBlock(child.DocComment); doc != nil {
						return doc
					}
				}
			}
		}
	}
	return nil
}

func fmtParams(params []symbols.ParamInfo) string {
	var parts []string
	for _, p := range params {
		s := ""
		if p.Type != "" {
			s += p.Type + " "
		}
		if p.IsVariadic {
			s += "..."
		}
		if p.IsReference {
			s += "&"
		}
		s += p.Name
		if p.DefaultValue != "" {
			s += " = " + p.DefaultValue
		}
		parts = append(parts, s)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func buildFQN(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "\\" + name
}

func getWordAt(source string, pos protocol.Position) string {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character > len(line) {
		return ""
	}
	// If cursor is on '$', include it and scan forward from the next char
	ch := pos.Character
	if ch < len(line) && line[ch] == '$' {
		start := ch
		end := ch + 1
		for end < len(line) && isWordChar(line[end]) {
			end++
		}
		if end > start+1 {
			return line[start:end]
		}
		return ""
	}
	start := pos.Character
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}
	if start > 0 && line[start-1] == '$' {
		start--
	}
	end := pos.Character
	for end < len(line) && isWordChar(line[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return line[start:end]
}

func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '\\'
}
