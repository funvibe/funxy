const { LanguageClient, TransportKind } = require('vscode-languageclient/node');
const vscode = require('vscode');
const path = require('path');
const fs = require('fs');
const cp = require('child_process');

let client;

function activate(context) {
    // Get configuration
    const config = vscode.workspace.getConfiguration('funxy');
    const serverCommand = config.get('lsp.path') || 'funxy-lsp';

    // Helper to check if command exists in PATH
    function checkCommandExists(command) {
        if (path.isAbsolute(command)) {
            return fs.existsSync(command);
        }
        try {
            const platform = process.platform === 'win32' ? 'where' : 'which';
            cp.execSync(`${platform} ${command}`);
            return true;
        } catch (error) {
            return false;
        }
    }

    // Check if binary exists
    if (!checkCommandExists(serverCommand)) {
        const message = `Funxy Language Server ('${serverCommand}') not found. Features like Go to Definition will be disabled.`;
        const downloadAction = 'Download LSP';
        const settingsAction = 'Configure Path';
        
        vscode.window.showInformationMessage(message, downloadAction, settingsAction).then(selection => {
            if (selection === downloadAction) {
                vscode.env.openExternal(vscode.Uri.parse('https://github.com/funvibe/funxy/releases'));
            } else if (selection === settingsAction) {
                vscode.commands.executeCommand('workbench.action.openSettings', 'funxy.lsp.path');
            }
        });
        return;
    }

    const serverOptions = {
        run: { command: serverCommand, transport: TransportKind.stdio },
        debug: { command: serverCommand, transport: TransportKind.stdio }
    };

    const clientOptions = {
        documentSelector: [{ scheme: 'file', language: 'funxy' }],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.{lang,funxy,fx}')
        }
    };

    client = new LanguageClient(
        'funxyLsp',
        'Funxy Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

function deactivate() {
    if (!client) {
        return undefined;
    }
    return client.stop();
}

module.exports = {
    activate,
    deactivate
};
