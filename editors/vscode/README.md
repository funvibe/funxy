# Funxy VS Code Extension

Official Visual Studio Code support for the [Funxy programming language](https://github.com/funvibe/funxy).

## Features

- **Syntax Highlighting**: Colorful syntax highlighting for `.lang`, `.funxy`, and `.fx` files.
- **Language Server Protocol (LSP)**:
  - **Hover**: Type information and documentation.
  - **Go to Definition**: Jump to variable, function, and type definitions.
  - **Diagnostics**: Real-time syntax and type error reporting.

## Installation

This extension requires the **Funxy Language Server** (`funxy-lsp`) to be installed on your system.

### Step 1: Install the Language Server

1. Go to the [Releases page](https://github.com/funvibe/funxy/releases).
2. Download the `funxy-lsp` binary for your OS.
3. Rename it to `funxy-lsp` (or `funxy-lsp.exe` on Windows).
4. On macOS/Linux, you may need to make it executable:
   ```bash
   chmod +x funxy-lsp
   ```
5. Place it in a folder included in your system's `PATH`.

### Step 2: Install the Extension

1. Download the `.vsix` file from the [Releases page](https://github.com/funvibe/funxy/releases).
2. Open VS Code.
3. Go to the Extensions view (`Ctrl+Shift+X` or `Cmd+Shift+X`).
4. Click the "..." menu in the top-right corner.
5. Select **Install from VSIX...**.
6. Choose the downloaded `funxy-language-x.x.x.vsix` file.

## Building from Source

To build the extension from source:

1. Install Node.js and npm.
2. Install the `vsce` tool globally:
   ```bash
   npm install -g @vscode/vsce
   ```
3. Install dependencies:
   ```bash
   cd editors/vscode
   npm install
   ```
4. Package the extension:
   ```bash
   vsce package
   ```
   This will create a `.vsix` file (e.g., `funxy-language-0.5.3.vsix`).

## Configuration

If `funxy-lsp` is not in your global PATH, you can specify its location:

1. Open VS Code Settings (`Cmd+,` or `Ctrl+,`).
2. Search for `funxy`.
3. Set **Funxy > Lsp: Path** to the absolute path of your binary (e.g., `/usr/local/bin/funxy-lsp`).

## Troubleshooting

- **"Funxy LSP binary not found"**: Ensure `funxy-lsp` is in your PATH or configured in settings.
- **No highlighting**: Ensure your file has a `.lang`, `.funxy`, or `.fx` extension.
