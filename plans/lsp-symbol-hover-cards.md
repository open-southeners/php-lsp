Here’s a solid hover card design for a VSCode PHP LSP that feels dense but still readable.

## Hover card goal

When hovering any PHP symbol, show:

1. **Primary symbol identity**
2. **Resolved signature / declaration**
3. **Short description from docblock**
4. **Parent definition summary** for classes, methods, and inherited members
5. **Parsed common docblock tags**
6. **Relevant PHP manual link** when the token maps to core PHP docs
7. **Fast navigation actions** such as go to definition / implementations if you want clickable command links in the hover

VS Code hovers support rich markdown, and trusted markdown can include command links, so this can be built as a structured `MarkdownString` rather than a full custom webview. ([code.visualstudio.com][1])

---

## Recommended visual structure

### 1. Header row

Use a compact top line with icon + token kind + fully qualified name.

Example:

```php
$(symbol-class) App\Services\UserImporter
```

Sub-line:

```text
class • namespace App\Services
```

For methods:

```php
$(symbol-method) UserImporter::import(array $rows): ImportResult
```

For properties:

```php
$(symbol-field) User::$email
```

For functions:

```php
$(symbol-function) array_map(?callable $callback, array $array, array ...$arrays): array
```

---

### 2. Primary declaration block

Always show a code block first. It is the most useful part of the hover.

#### Class example

```php
final class UserImporter extends BaseImporter implements ImporterContract
```

#### Method example

```php
public function import(array $rows, bool $dryRun = false): ImportResult
```

#### Property example

```php
protected ?string $email = null
```

#### Constant example

```php
public const STATUS_ACTIVE = 'active'
```

---

### 3. Summary / description

Use the first meaningful sentence from the docblock summary.

Example:

> Imports users from a normalized row payload and returns a typed import report.

Rules:

* Strip markdown noise
* Collapse whitespace
* Keep to 1–3 lines
* Fall back to “No documentation available” only if nothing exists

---

### 4. Parsed docblock metadata

This is where the hover becomes much more useful than a plain definition preview.

Show only the common tags that matter most:

* `@param`
* `@return`
* `@throws`
* `@deprecated`
* `@template`
* `@extends`
* `@implements`
* `@mixin`
* `@var`
* `@property`, `@property-read`, `@property-write`
* `@see`

#### Method hover example

```text
Params
• $rows array — Normalized input rows.
• $dryRun bool — Validate only, do not persist.

Returns
• ImportResult — Import summary and per-row failures.

Throws
• ValidationException — When a row schema is invalid.
• ImportTransportException — When persistence fails.
```

#### Class hover example

```text
Extends
• BaseImporter<TModel>

Implements
• ImporterContract

Templates
• TModel of Model
```

---

### 5. Parent definition section

This is the part you specifically asked for.

For classes:

* show the **direct parent class declaration first line only**
* parse **common docblock tags** from the parent class too

#### Example

```text
Parent
BaseImporter<TModel>

Definition
abstract class BaseImporter extends Service

Parent docs
• @template TModel of Model
• @property-read Connection $connection
• @mixin HandlesTransactions
```

For methods:

* if the method overrides or inherits, show the parent/interface declaration first line only
* include parsed tags from the inherited docblock if the local one is missing or partial

#### Example

```text
Inherited from
ImporterContract::import

Definition
public function import(array $rows, bool $dryRun = false): ImportResult
```

For properties:

* show where the property was originally declared

#### Example

```text
Declared in parent
BaseUser::$email

Definition
protected ?string $email
```

This should clearly distinguish:

* **declared here**
* **inherited**
* **overridden**
* **implements interface contract**

---

### 6. PHP manual link section

When the hovered token resolves to a built-in PHP symbol, add a footer link.

Examples:

* functions: `parse_url`, `array_map`, `str_contains`
* classes/interfaces: `DateTimeImmutable`, `Throwable`, `Traversable`
* extensions/functions grouped under PHP manual pages

The PHP manual is the canonical reference for PHP’s online documentation and function reference. ([php.net][2])

#### Footer examples

```text
PHP Manual: parse_url
```

or as markdown:

```md
[PHP Manual](https://www.php.net/manual/en/function.parse-url.php)
```

For built-ins, resolve links using known manual page conventions where possible, such as function pages like `function.parse-url.php`. That exact pattern is used by PHP manual pages such as `parse_url`. ([php.net][3])

---

## Best card layouts by token type

## A. Class / interface / trait / enum

Recommended order:

1. Header
2. Declaration
3. Summary
4. Extends / implements / uses
5. Parent definition first line
6. Parent docblock parsed tags
7. Source location
8. PHP manual link if built-in

### Example

````md
$(symbol-class) `App\Domain\Billing\InvoiceGenerator`

```php
final class InvoiceGenerator extends AbstractGenerator implements GeneratorContract
````

Generates fiscal-compliant invoices for tenant-scoped billing flows.

**Extends**

* `AbstractGenerator`

**Implements**

* `GeneratorContract`

**Parent**
`abstract class AbstractGenerator extends Service`

**Parent docs**

* `@template TPayload of array`
* `@mixin FormatsMoney`

[Go to Definition](command:editor.action.revealDefinition) • [Find References](command:editor.action.referenceSearch.trigger)

````

Command URIs in hover markdown are supported when the markdown is trusted. :contentReference[oaicite:3]{index=3}

---

## B. Method

Recommended order:

1. Header
2. Declaration
3. Summary
4. Params / return / throws
5. Override/inherit info
6. Parent method first line
7. PHP docs link if built-in

### Example

```md
$(symbol-method) `Collection::map`

```php
public function map(callable $callback): static
````

Transform each item in the collection and return a new collection instance.

**Params**

* `$callback callable` — Receives item and key.

**Returns**

* `static`

**Inherited contract**
`Enumerable::map`

**Definition**
`public function map(callable $callback): static`

````

---

## C. Function

Recommended order:

1. Header
2. Declaration
3. Summary
4. Params / return
5. PHP docs link
6. Extension/category if known

### Example

```md
$(symbol-function) `parse_url`

```php
parse_url(string $url, int $component = -1): int|string|array|null|false
````

Parse a URL and return its components.

**Params**

* `$url string`
* `$component int = -1`

**Returns**

* `int|string|array|null|false`

[PHP Manual](https://www.php.net/manual/en/function.parse-url.php)

````

That signature and manual page are directly reflected in the PHP docs. :contentReference[oaicite:4]{index=4}

---

## D. Property

Recommended order:

1. Header
2. Declaration
3. Summary
4. `@var`
5. Declared-in / inherited-from
6. Parent declaration first line if inherited

---

## E. Namespace / use-import / constant / keyword

Not every token deserves the same richness.

- **Namespace**: show resolved namespace and file path
- **Imported class (`use`)**: show final FQCN and class summary
- **Class constant**: show value preview and declaring class
- **Language keywords**: keep minimal; maybe no hover unless the parser can attach meaningful info
- **Docblock annotations**: resolve linked symbol if possible, otherwise plain description

---

## Parsing rules I recommend

## First-line parent definition

For the parent definition preview, only extract the **first declaration line**, not the whole body.

Examples:
- `abstract class BaseImporter extends Service`
- `interface Arrayable`
- `public function toArray(): array`
- `protected ?string $email`

This keeps the hover compact.

---

## Common docblock parsing

Normalize these into structured sections:

### For classes
- `@template`
- `@extends`
- `@implements`
- `@mixin`
- `@property*`
- `@method`

### For methods/functions
- `@param`
- `@return`
- `@throws`
- `@deprecated`
- `@see`

### For properties
- `@var`
- `@deprecated`
- `@see`

### Parsing behavior
- Prefer AST/signature types over docblock types when both exist
- Use docblock types to supplement generics, array shapes, template bounds, and descriptions
- If local symbol lacks docs, inherit docs from overridden parent/interface symbol and label them as inherited

---

## Ranking what to show

Because hover space is limited, use this priority order:

1. Declaration
2. Description
3. Params/return or extends/implements
4. Parent definition
5. Parent docblock
6. External links
7. Extra actions

Hide less important sections unless present.

---

## Link strategy for PHP docs

Use three resolution tiers:

### Tier 1: Exact built-in function/class map
Maintain a lookup table for known internal symbols to exact manual URLs.

Best for:
- `parse_url`
- `DateTimeImmutable`
- `PDO`
- `Throwable`

### Tier 2: Slug generation
For functions, generate likely manual URLs like:

- `parse_url` → `function.parse-url.php`

This matches PHP manual naming for functions like `parse_url`. :contentReference[oaicite:5]{index=5}

### Tier 3: Fallback search/manual index
If exact page unknown, link to manual search or extension reference pages. PHP’s manual has a function index and broader function reference by extension/category. :contentReference[oaicite:6]{index=6}

---

## UX details that will make it feel polished

- Use icons: `$(symbol-class)`, `$(symbol-method)`, `$(symbol-function)`, etc.
- Bold section labels only, not everything
- Keep code fences for signatures
- Keep max hover height sensible by truncating long docs
- Use separators only between major sections
- De-emphasize file paths and source info
- Show inherited sections in muted wording like “Inherited from” or “Parent”
- Cache parent symbol resolution so hovers feel instant

VS Code markdown in hovers also supports theme icons, which helps make these cards feel native. :contentReference[oaicite:7]{index=7}

---

## Recommended final hover template

```md
$(symbol-kind) `Fully\Qualified\SymbolName`

```php
<primary declaration>
````

<summary sentence>

**Params**

* ...

**Returns**

* ...

**Throws**

* ...

**Parent**
`<parent symbol name>`

**Definition**
`<first line only>`

**Parent docs**

* `@template ...`
* `@property-read ...`

[PHP Manual](...) • [Go to Definition](command:editor.action.revealDefinition)

```

---

## My recommendation

For the best result, make the hover card use **three modes** internally:

- **Compact** for keywords/imports/constants
- **Standard** for functions/properties
- **Rich** for classes and methods with inheritance

That prevents every hover from becoming too noisy while still giving class and method references the deeper parent/docblock context you want.

I can turn this into a concrete JSON-like hover schema or a TypeScript `HoverProvider` rendering example next.
::contentReference[oaicite:8]{index=8}
```

[1]: https://code.visualstudio.com/api/extension-guides/command?utm_source=chatgpt.com "Commands | Visual Studio Code Extension API"
[2]: https://www.php.net/manual/en/index.php?utm_source=chatgpt.com "PHP Manual"
[3]: https://www.php.net/manual/en/function.parse-url.php?utm_source=chatgpt.com "parse_url - Manual"
