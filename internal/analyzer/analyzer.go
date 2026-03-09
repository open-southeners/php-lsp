package analyzer

import (
	"regexp"
	"strings"

	"github.com/open-southeners/php-lsp/internal/container"
	"github.com/open-southeners/php-lsp/internal/parser"
	"github.com/open-southeners/php-lsp/internal/protocol"
	"github.com/open-southeners/php-lsp/internal/symbols"
)

type Analyzer struct {
	index     *symbols.Index
	container *container.ContainerAnalyzer
}

func NewAnalyzer(index *symbols.Index, ca *container.ContainerAnalyzer) *Analyzer {
	return &Analyzer{index: index, container: ca}
}

func (a *Analyzer) FindDefinition(uri, source string, pos protocol.Position) *protocol.Location {
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

	// Handle $variable → go to its type definition
	if strings.HasPrefix(word, "$") {
		return a.definitionForVariable(word, file, source, pos)
	}

	// Find the start of the word on the line
	wordStart := pos.Character
	for wordStart > 0 && isWordChar(line[wordStart-1]) {
		wordStart--
	}

	// Check for -> or :: access (method/property on a class)
	if classFQN := a.resolveAccessChain(line, wordStart, file, source, pos); classFQN != "" {
		if sym := a.findMember(classFQN, word); sym != nil {
			return symbolLocation(sym)
		}
	}

	// Resolve via use statements
	if file != nil {
		for _, u := range file.Uses {
			if u.Alias == word {
				if sym := a.index.Lookup(u.FullName); sym != nil {
					return symbolLocation(sym)
				}
			}
		}
		// Try as class name in current namespace
		fqn := a.resolveClassName(word, file)
		if fqn != word {
			if sym := a.index.Lookup(fqn); sym != nil {
				return symbolLocation(sym)
			}
		}
	}

	// FQN with backslashes
	if strings.Contains(word, "\\") {
		if sym := a.index.Lookup(word); sym != nil {
			return symbolLocation(sym)
		}
	}

	// Fallback: lookup by short name
	lookupName := word
	if idx := strings.LastIndex(word, "\\"); idx >= 0 {
		lookupName = word[idx+1:]
	}
	for _, sym := range a.index.LookupByName(lookupName) {
		if sym.URI != "builtin" {
			return symbolLocation(sym)
		}
	}

	return nil
}

// definitionForVariable resolves a $variable to its type's definition.
func (a *Analyzer) definitionForVariable(varName string, file *parser.FileNode, source string, pos protocol.Position) *protocol.Location {
	if file == nil {
		return nil
	}
	typeName := a.resolveVariableType(varName, file, source, pos)
	if typeName == "" {
		return nil
	}
	if sym := a.index.Lookup(typeName); sym != nil {
		return symbolLocation(sym)
	}
	return nil
}

// resolveAccessChain walks left through -> and :: chains to return the FQN
// of the class that owns the member at wordStart.
func (a *Analyzer) resolveAccessChain(line string, wordStart int, file *parser.FileNode, source string, pos protocol.Position) string {
	i := wordStart
	for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
		i--
	}
	if i < 2 {
		return ""
	}

	if line[i-2] == '-' && line[i-1] == '>' {
		i -= 2
	} else if line[i-2] == ':' && line[i-1] == ':' {
		i -= 2
	} else {
		return ""
	}

	for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
		i--
	}

	// Skip past closing paren for method chains
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
		for i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
			i--
		}
	}

	end := i
	for i > 0 && isWordChar(line[i-1]) {
		i--
	}
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

	switch target {
	case "$this", "self", "static":
		return a.findEnclosingClass(file, pos)
	case "parent":
		classFQN := a.findEnclosingClass(file, pos)
		if classFQN == "" {
			return ""
		}
		chain := a.index.GetInheritanceChain(classFQN)
		if len(chain) > 0 {
			return chain[0]
		}
		return ""
	}

	if strings.HasPrefix(target, "$") {
		return a.resolveVariableType(target, file, source, pos)
	}

	// Try as a class name (for static access like Logger::create)
	if fqn := a.resolveClassName(target, file); fqn != "" {
		if a.index.Lookup(fqn) != nil {
			return fqn
		}
	}

	// Recursive chain resolution
	ownerFQN := a.resolveAccessChain(line, i, file, source, pos)
	if ownerFQN == "" {
		return ""
	}
	member := a.findMember(ownerFQN, target)
	if member == nil {
		return ""
	}
	return a.memberType(member, file)
}

// resolveVariableType infers the type of a variable from context.
func (a *Analyzer) resolveVariableType(varName string, file *parser.FileNode, source string, pos protocol.Position) string {
	// Check enclosing method parameters
	enclosingMethod := a.findEnclosingMethod(file, pos)
	if enclosingMethod != nil {
		for _, param := range enclosingMethod.Params {
			if param.Name == varName {
				return a.resolveClassName(param.Type.Name, file)
			}
		}
	}

	// Check class properties
	for _, cls := range file.Classes {
		for _, prop := range cls.Properties {
			if "$"+prop.Name == varName && prop.Type.Name != "" {
				return a.resolveClassName(prop.Type.Name, file)
			}
		}
	}

	lines := strings.Split(source, "\n")
	bare := strings.TrimPrefix(varName, "$")

	// Look for $var = new ClassName(...)
	newPattern := regexp.MustCompile(`\$` + regexp.QuoteMeta(bare) + `\s*=\s*new\s+([A-Za-z_\\]+)`)
	for i := pos.Line; i >= 0 && i >= pos.Line-200; i-- {
		if i >= len(lines) {
			continue
		}
		if m := newPattern.FindStringSubmatch(lines[i]); m != nil {
			return a.resolveClassName(m[1], file)
		}
	}

	// Check @var annotations
	varDocPattern := regexp.MustCompile(`@var\s+([A-Za-z_\\]+)\s+\$` + regexp.QuoteMeta(bare) + `\b`)
	for i := pos.Line; i >= 0 && i >= pos.Line-5; i-- {
		if i >= len(lines) {
			continue
		}
		if m := varDocPattern.FindStringSubmatch(lines[i]); m != nil {
			return a.resolveClassName(m[1], file)
		}
	}

	return ""
}

func (a *Analyzer) findEnclosingClass(file *parser.FileNode, pos protocol.Position) string {
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

func (a *Analyzer) findEnclosingMethod(file *parser.FileNode, pos protocol.Position) *parser.MethodNode {
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

func (a *Analyzer) resolveClassName(name string, file *parser.FileNode) string {
	if name == "" {
		return ""
	}
	if file == nil {
		return name
	}
	if strings.HasPrefix(name, "\\") {
		return strings.TrimPrefix(name, "\\")
	}
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
		if a.index.Lookup(fqn) != nil {
			return fqn
		}
	}
	return name
}

func (a *Analyzer) findMember(classFQN, memberName string) *symbols.Symbol {
	members := a.index.GetClassMembers(classFQN)
	for _, m := range members {
		if m.Name == memberName || m.Name == "$"+memberName {
			return m
		}
	}
	return nil
}

func (a *Analyzer) memberType(member *symbols.Symbol, file *parser.FileNode) string {
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
	if typeName == "self" || typeName == "static" {
		return member.ParentFQN
	}
	return a.resolveClassName(typeName, file)
}

func symbolLocation(sym *symbols.Symbol) *protocol.Location {
	if sym.URI == "" || sym.URI == "builtin" {
		return nil
	}
	return &protocol.Location{URI: sym.URI, Range: sym.Range}
}

func buildFQN(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "\\" + name
}

func (a *Analyzer) FindReferences(uri, source string, pos protocol.Position) []protocol.Location {
	word := getWordAt(source, pos)
	if word == "" {
		return nil
	}
	var locs []protocol.Location
	for _, sym := range a.index.LookupByName(word) {
		if sym.URI != "builtin" {
			locs = append(locs, protocol.Location{URI: sym.URI, Range: sym.Range})
		}
		if sym.Kind == symbols.KindInterface {
			for _, impl := range a.index.GetImplementors(sym.FQN) {
				if impl.URI != "builtin" {
					locs = append(locs, protocol.Location{URI: impl.URI, Range: impl.Range})
				}
			}
		}
	}
	return locs
}

func (a *Analyzer) GetDocumentSymbols(uri, source string) []protocol.DocumentSymbol {
	file := parser.ParseFile(source)
	if file == nil {
		return nil
	}
	var ds []protocol.DocumentSymbol
	for _, cls := range file.Classes {
		s := protocol.DocumentSymbol{Name: cls.Name, Kind: protocol.SymbolKindClass,
			Range: mkRange(cls.StartLine), SelectionRange: mkRange(cls.StartLine)}
		for _, m := range cls.Methods {
			s.Children = append(s.Children, protocol.DocumentSymbol{Name: m.Name, Detail: m.Visibility, Kind: protocol.SymbolKindMethod, Range: mkRange(m.StartLine), SelectionRange: mkRange(m.StartLine)})
		}
		for _, p := range cls.Properties {
			s.Children = append(s.Children, protocol.DocumentSymbol{Name: p.Name, Detail: p.Type.Name, Kind: protocol.SymbolKindProperty, Range: mkRange(p.StartLine), SelectionRange: mkRange(p.StartLine)})
		}
		ds = append(ds, s)
	}
	for _, iface := range file.Interfaces {
		s := protocol.DocumentSymbol{Name: iface.Name, Kind: protocol.SymbolKindInterface, Range: mkRange(iface.StartLine), SelectionRange: mkRange(iface.StartLine)}
		ds = append(ds, s)
	}
	for _, en := range file.Enums {
		ds = append(ds, protocol.DocumentSymbol{Name: en.Name, Kind: protocol.SymbolKindEnum, Range: mkRange(en.StartLine), SelectionRange: mkRange(en.StartLine)})
	}
	for _, fn := range file.Functions {
		ds = append(ds, protocol.DocumentSymbol{Name: fn.Name, Kind: protocol.SymbolKindFunction, Range: mkRange(fn.StartLine), SelectionRange: mkRange(fn.StartLine)})
	}
	return ds
}

func (a *Analyzer) GetSignatureHelp(uri, source string, pos protocol.Position) *protocol.SignatureHelp {
	line := getLineAt(source, pos.Line)
	if line == "" {
		return nil
	}
	prefix := line[:min(pos.Character, len(line))]
	funcName, activeParam := extractCallInfo(prefix)
	if funcName == "" {
		return nil
	}
	syms := a.index.LookupByName(funcName)
	if len(syms) == 0 {
		return nil
	}
	sym := syms[0]
	sig := protocol.SignatureInformation{Label: sym.Name + formatParamLabel(sym)}
	for _, p := range sym.Params {
		l := ""
		if p.Type != "" {
			l = p.Type + " "
		}
		l += p.Name
		sig.Parameters = append(sig.Parameters, protocol.ParameterInformation{Label: l})
	}
	return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{sig}, ActiveParameter: activeParam}
}

func extractCallInfo(prefix string) (string, int) {
	activeParam := 0
	depth := 0
	parenPos := -1
	for i := len(prefix) - 1; i >= 0; i-- {
		switch prefix[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				parenPos = i
				goto found
			}
			depth--
		case ',':
			if depth == 0 {
				activeParam++
			}
		}
	}
	return "", 0
found:
	if parenPos <= 0 {
		return "", 0
	}
	end := parenPos
	start := end - 1
	for start >= 0 && isWordChar(prefix[start]) {
		start--
	}
	start++
	if start >= end {
		return "", 0
	}
	return prefix[start:end], activeParam
}

func formatParamLabel(sym *symbols.Symbol) string {
	var ps []string
	for _, p := range sym.Params {
		s := ""
		if p.Type != "" {
			s = p.Type + " "
		}
		s += p.Name
		ps = append(ps, s)
	}
	ret := sym.ReturnType
	if ret == "" {
		ret = "mixed"
	}
	return "(" + strings.Join(ps, ", ") + "): " + ret
}

func mkRange(line int) protocol.Range {
	return protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}}
}

func getLineAt(source string, line int) string {
	lines := strings.Split(source, "\n")
	if line >= 0 && line < len(lines) {
		return lines[line]
	}
	return ""
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
	// Handle cursor on '$'
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
