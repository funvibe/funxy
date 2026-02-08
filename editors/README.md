# Funxy Syntax Highlighting

## VSCode Extension

### Install from folder

```bash
cp -r editors/vscode ~/.vscode/extensions/funxy-language
```

Restart VSCode and open any `.lang` or `.funxy` file.

### Build VSIX package

```bash
cd editors/vscode
npm install -g @vscode/vsce
vsce package
code --install-extension funxy-language-0.5.5.vsix
```

## Sublime Text

### Install

Copy files to Sublime packages directory:

**macOS:**
```bash
cp -r editors/sublime ~/Library/Application\ Support/Sublime\ Text/Packages/Funxy
```

**Linux:**
```bash
cp -r editors/sublime ~/.config/sublime-text/Packages/Funxy
```

**Windows:**
```bash
copy editors\sublime %APPDATA%\Sublime Text\Packages\Funxy
```

Restart Sublime Text. Files with `.lang`, `.funxy`, `.fx` extensions will be highlighted.

## Web (Markdown / Documentation)

### Prism.js (Docusaurus, VitePress, Jekyll)

```html
<link href="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/themes/prism-tomorrow.min.css" rel="stylesheet">
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/prism.min.js"></script>
<script src="prism-funxy.js"></script>
```

File: `examples/playground/prism-funxy.js`

### highlight.js

```html
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css">
<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js"></script>
<script src="hljs-funxy.js"></script>
<script>hljs.highlightAll();</script>
```

File: `examples/playground/hljs-funxy.js`

### CodeMirror (Interactive editors)

File: `examples/playground/playground.html` — contains full CodeMirror mode definition

## Supported Tokens

| Token | Examples | Scope |
|-------|----------|-------|
| Keywords | `fun`, `type`, `match`, `if`, `for`, `trait` | `keyword.control` |
| Types | `Int`, `String`, `List<T>`, `MyType` | `entity.name.type` |
| Functions | `greet(`, `processData(` | `entity.name.function` |
| Strings | `"hello"`, `"${name}"` | `string.quoted.double` |
| Chars | `'a'`, `'\n'` | `string.quoted.single` |
| Bytes | `@"data"`, `@x"FF"`, `@b"01"` | `string.quoted.other` |
| Bits | `#b"01"`, `#x"FF"`, `#o"77"` | `string.quoted.other` |
| Format | `%"%.2f"`, `%"Hello %s"` | `string.quoted.other` |
| Numbers | `42`, `3.14`, `0xFF`, `100n` | `constant.numeric` |
| Operators | `->`, `|>`, `>>=`, `++`, `::` | `keyword.operator` |
| Comments | `// comment` | `comment.line` |
| Builtins | `print`, `show`, `map`, `filter` | `support.function` |

## File Structure

```
editors/
├── vscode/
│   ├── package.json
│   ├── funxy.tmLanguage.json
│   └── language-configuration.json
├── sublime/
│   ├── Funxy.sublime-syntax
│   ├── Funxy.sublime-settings
│   └── Comments.tmPreferences
└── README.md

examples/playground/
├── prism-funxy.js          # Prism.js language definition
├── hljs-funxy.js           # highlight.js language definition
├── prism-example.html      # Prism usage example
└── SYNTAX_HIGHLIGHTING.md  # Detailed integration guide
```

## GitHub/GitLab

GitHub doesn't support custom languages yet. Workarounds:

1. Use similar syntax: ` ```rust ` or ` ```haskell ` (partial highlighting)
2. Submit to [Linguist](https://github.com/github/linguist) for official support
