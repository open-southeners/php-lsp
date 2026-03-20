# PHP LSP — Go-based Language Server for PHP 8.5+

[![CI](https://github.com/open-southeners/php-lsp/actions/workflows/test.yml/badge.svg)](https://github.com/open-southeners/php-lsp/actions/workflows/test.yml)
[![Release](https://github.com/open-southeners/php-lsp/actions/workflows/release.yml/badge.svg)](https://github.com/open-southeners/php-lsp/actions/workflows/release.yml)
[![VS Code Marketplace](https://img.shields.io/visual-studio-marketplace/v/open-southeners.php-lsp?label=VS%20Code%20Marketplace)](https://marketplace.visualstudio.com/items?itemName=open-southeners.php-lsp)
[![Open VSX](https://img.shields.io/open-vsx/v/open-southeners/php-lsp?label=Open%20VSX)](https://open-vsx.org/extension/open-southeners/php-lsp)
[![GitHub Release](https://img.shields.io/github/v/release/open-southeners/php-lsp)](https://github.com/open-southeners/php-lsp/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![PHP 8.0–8.5](https://img.shields.io/badge/PHP-8.0–8.5-777BB4?logo=php&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)

A high-performance Language Server Protocol implementation written in **Go** for PHP 8.5+, with deep understanding of **Laravel** and **Symfony** dependency injection containers.

## Features

### Language Intelligence
- **Full PHP 8.0–8.5 Tokenizer & Parser** — Union types, intersection types, DNF types, enums, fibers, readonly classes, property hooks, asymmetric visibility, pipe operator `|>`
- **Context-Aware Completion** — Member access (`->`), static access (`::`), nullsafe (`?->`), `new`, `use`, type hints, variables, PHP 8 attributes, pipe targets
- **Hover Information** — Rich markdown with type signatures, inheritance chains, docblocks
- **Go to Definition / Find References** — Jump to and find all usages of classes, methods, functions
- **Document Symbols** — Full outline with classes, interfaces, enums, methods, properties
- **Signature Help** — Parameter hints while typing function/method calls
- **Diagnostics** — Deprecated functions, structural errors, unused imports

### Container-Aware Intelligence
- **Laravel** — Resolves `app()`, `bind()`, `singleton()`, facade accessors, service providers. 25+ pre-loaded framework bindings.
- **Symfony** — Parses `services.yaml`, PHP service configs, `#[Autowire]`, `#[AsController]`, `#[AsCommand]`, `#[AsEventListener]`, `#[AsMessageHandler]`, and auto-wiring from `src/`.
- **Constructor Injection** — Hover over type-hinted params to see what the container injects.
- **Interface → Concrete Resolution** — See which implementation is bound to an interface.

### PHP 8.5 Support
- Pipe operator `|>` with callable target completion
- All PHP 8.0–8.4 features: enums, fibers, readonly, match, named arguments, attributes, property hooks, asymmetric visibility, `#[\Override]`, typed constants

## Installation

### Editor Extensions (Recommended)

#### VS Code / VS Codium

Install from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=open-southeners.php-lsp) or [Open VSX Registry](https://open-vsx.org/extension/open-southeners/php-lsp):

```
ext install open-southeners.php-lsp
```

Or search for **"PHP LSP"** in the Extensions panel.

The extension bundles the language server binary — no additional setup is required. If you prefer using your own binary, set `phpLsp.executablePath` in your settings.

#### Zed

Install from the Zed extension registry — search for **"PHP LSP"** in `zed: extensions`.

The extension automatically downloads the correct binary for your platform from GitHub releases.

#### Neovim (lspconfig)

Install the `php-lsp` binary (see below), then add to your Neovim config:

```lua
require('lspconfig.configs').php_lsp = {
  default_config = {
    cmd = { 'php-lsp', '--transport', 'stdio' },
    filetypes = { 'php' },
    root_dir = require('lspconfig.util').root_pattern('composer.json', '.git'),
  }
}
require('lspconfig').php_lsp.setup({})
```

#### Any LSP Client

The server communicates over **stdio** using standard JSON-RPC 2.0:

```bash
php-lsp --transport stdio
php-lsp --transport stdio --log /tmp/php-lsp.log
```

### Binary Installation

#### Download from Releases

Grab the latest binary for your platform from [GitHub Releases](https://github.com/open-southeners/php-lsp/releases/latest).

Available platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

#### Quick Install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/open-southeners/php-lsp/main/scripts/install.sh | bash
```

#### From Source (requires Go 1.22+)

```bash
git clone https://github.com/open-southeners/php-lsp.git
cd php-lsp
make install
```

## Configuration

### VS Code Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `phpLsp.enable` | `true` | Enable/disable the extension |
| `phpLsp.executablePath` | `""` | Custom path to the php-lsp binary |
| `phpLsp.phpVersion` | `"8.5"` | Target PHP version (`8.0`–`8.5`) |
| `phpLsp.framework` | `"auto"` | Framework detection: `auto`, `laravel`, `symfony`, `none` |
| `phpLsp.containerAware` | `true` | Enable DI container analysis |
| `phpLsp.diagnostics.enable` | `true` | Enable diagnostics (static checks, PHPStan, Pint) |
| `phpLsp.diagnostics.phpstan.enable` | `true` | Enable PHPStan analysis on save |
| `phpLsp.diagnostics.phpstan.level` | `""` | PHPStan level (uses project config if empty) |
| `phpLsp.diagnostics.pint.enable` | `true` | Enable Laravel Pint on save |
| `phpLsp.maxIndexFiles` | `10000` | Maximum PHP files to index |
| `phpLsp.excludePaths` | `["vendor", ...]` | Paths to skip when indexing |

### Project Configuration

Create `.php-lsp.json` in your project root:

```json
{
  "phpVersion": "8.5",
  "framework": "auto",
  "excludePaths": ["vendor", "node_modules", ".git"],
  "containerAware": true,
  "diagnosticsEnabled": true,
  "maxIndexFiles": 10000
}
```

Framework is auto-detected from `artisan` (Laravel), `bin/console` (Symfony), or `composer.json` requires.

## How Container Analysis Works

### Laravel
Scans `app/Providers/*.php` for `$this->app->bind()` and `$this->app->singleton()` calls. Pre-loads 25+ core framework bindings (auth, cache, config, db, events, filesystem, queue, router, session, view, etc.).

### Symfony
Parses `config/services.yaml`, XML service definitions, and PHP config files. Auto-wires classes in `src/` and resolves interface bindings from `implements` declarations. Understands Symfony attributes like `#[AsController]`, `#[AsCommand]`, `#[Autowire]`.

## Architecture

```
cmd/php-lsp/          Entry point (stdio JSON-RPC transport)
internal/
  protocol/           LSP type definitions
  config/             Configuration + framework auto-detection
  parser/             PHP tokenizer + lightweight AST parser
  symbols/            Symbol table, index, built-in stubs
  container/          DI container analyzer (Laravel + Symfony)
  completion/         Context-aware completion provider
  hover/              Hover information provider
  diagnostics/        Real-time diagnostics
  analyzer/           Go-to-definition, references, document symbols, signature help
  lsp/                JSON-RPC server, message dispatch
editors/
  vscode/             VS Code extension (TypeScript + vscode-languageclient)
  zed/                Zed extension (Rust + zed_extension_api)
scripts/              Build + install scripts
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and contribution guidelines.

## License

[MIT](LICENSE)
