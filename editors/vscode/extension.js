// Karkain Language Support — toolchain-wiring foundation.
//
// This extension shells out to the `karkain` CLI for check / compile / run /
// format operations. It is a foundation: command wiring and syntax awareness
// are real, and behavior is kept deterministic and honest ("karkain not
// installed" is reported instead of faked). Language intelligence (autocomplete,
// semantic highlighting, goto-definition) is provided by the language server
// and will be wired here as it matures.

const vscode = require('vscode');

// Diagnostic schema v1 mirrors `karkain check --format=json`:
// [{ "file","line","column","severity","code","message" }]
function compilerPath() {
  const c = vscode.workspace.getConfiguration('karkain');
  return String(c.get('compilerPath') || 'karkain');
}

function activeKarkainDocument() {
  const doc = vscode.window.activeTextEditor && vscode.window.activeTextEditor.document;
  if (!doc) {
    vscode.window.showWarningMessage('Karkain: open a .kark file first.');
    return null;
  }
  if (doc.languageId !== 'karkain') {
    vscode.window.showWarningMessage('Karkain: the active file is not a .kark file.');
    return null;
  }
  return doc;
}

function runInTerminal(label, args) {
  const term = vscode.window.createTerminal({ name: label });
  term.show(true);
  const quoted = args.map((a) => '"' + a.replace(/"/g, '\\"') + '"').join(' ');
  term.sendText(compilerPath() + ' ' + quoted);
}

async function runCheck(doc) {
  const { spawn } = require('child_process');
  const proc = spawn(compilerPath(), ['check', '--format=json', doc.uri.fsPath], {
    cwd: vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0]
      ? vscode.workspace.workspaceFolders[0].uri.fsPath
      : undefined,
  });
  let out = '';
  proc.stdout.on('data', (d) => { out += String(d); });
  const fail = await new Promise((resolve) => {
    proc.stderr.on('data', () => {});
    proc.on('error', (err) => resolve(String(err && err.message)));
    proc.on('close', (code) => resolve(code === undefined || code === null ? 'unknown' : code));
  });
  const diags = parseDiagnostics(out);
  if (diags.length === 0 && fail === 0) {
    vscode.window.setStatusBarMessage('Karkain: check passed', 3000);
    return;
  }
  if (diags.length === 0) {
    vscode.window.showErrorMessage('Karkain: check failed to run (' + fail + '). Is `karkain` on PATH?');
    return;
  }
  const text = diags.map((d) => d.line + ':' + d.column + ' ' + d.code + ' ' + d.message).join('\n');
  vscode.window.showErrorMessage('Karkain: ' + diags.length + ' diagnostic(s)\n' + text);
}

function parseDiagnostics(out) {
  try {
    const arr = JSON.parse(out);
    if (Array.isArray(arr)) {
      return arr;
    }
  } catch (_e) { /* not JSON */ }
  return [];
}

function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand('karkain.check', async () => {
      const doc = activeKarkainDocument();
      if (doc) { await runCheck(doc); }
    }),

    vscode.commands.registerCommand('karkain.compile', () => {
      const doc = activeKarkainDocument();
      if (doc) { runInTerminal('karkain build', ['build', doc.uri.fsPath]); }
    }),

    vscode.commands.registerCommand('karkain.run', () => {
      const doc = activeKarkainDocument();
      if (doc) { runInTerminal('karkain run', ['run', doc.uri.fsPath]); }
    }),

    vscode.languages.registerDocumentFormattingEditProvider('karkain', {
      async provideDocumentFormattingEdits(document) {
        const { execFile } = require('child_process');
        const path = document.uri.fsPath;
        const formatted = await new Promise((resolve) => {
          execFile(compilerPath(), ['fmt', path], { encoding: 'utf8' }, (err, stdout) => {
            resolve(stdout ? String(stdout) : null);
          });
        });
        if (formatted === null) {
          vscode.window.showErrorMessage('Karkain: formatter failed. Is `karkain` on PATH?');
          return null;
        }
        return [vscode.TextEdit.replace(
          new vscode.Range(
            document.positionAt(0),
            document.positionAt(document.getText().length)
          ),
          formatted
        )];
      },
    }),

    vscode.commands.registerCommand('karkain.formatDocument', () => {
      if (vscode.window.activeTextEditor) {
        vscode.commands.executeCommand('editor.action.formatDocument');
      }
    })
  );

  vscode.workspace.onDidSaveTextDocument((doc) => {
    if (doc.languageId === 'karkain' && vscode.workspace.getConfiguration('karkain').get('formatOnSave')) {
      vscode.commands.executeCommand('editor.action.formatDocument');
    }
  });
}

function deactivate() {}

module.exports = { activate, deactivate };