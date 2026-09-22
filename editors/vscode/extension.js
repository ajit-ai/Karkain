// Karkain Language Support — toolchain wiring + language client.
//
// Shell-out commands (check / compile / run / format) stay as-is. Language
// intelligence (semantic highlighting, hover, go-to-definition,
// autocompletion) is served by `karkain lsp` over stdio and wired here
// through a vscode-languageclient LanguageClient started on activation.
// If the client library is not installed (`npm install` in this directory),
// the shell-out commands keep working and activation logs one warning.
const vscode = require('vscode');

let karkainClient = null;

function startLanguageClient(context) {
  let lc;
  try {
    lc = require('vscode-languageclient/node');
  } catch (_e) {
    console.warn('Karkain: vscode-languageclient not installed; language intelligence disabled (shell commands still work).');
    return;
  }
  const serverOptions = {
    command: compilerPath(),
    args: ['lsp'],
    options: {},
  };
  const clientOptions = {
    documentSelector: [{ scheme: 'file', language: 'karkain' }],
  };
  karkainClient = new lc.LanguageClient('karkain-lsp', 'Karkain Language Server', serverOptions, clientOptions);
  context.subscriptions.push(karkainClient.start());
}

// Diagnostic schema v1 mirrors `karkain check --format=json`:
// [{ "file","line","column","severity","code","message" }]
function compilerPath() {
  const c = vscode.workspace.getConfiguration('karkain');
  return String(c.get('compilerPath') || 'karkain');
}

function debuggerPath() {
  const c = vscode.workspace.getConfiguration('karkain');
  return String(c.get('debuggerPath') || 'gdb');
}

// runDebug implements the `karkain.debug` command (Phase 140): build the
// active .kark file with debug symbols (`karkain build -g`), then start a
// cppdbg/gdb session on the produced binary. The build must succeed first —
// starting the debugger on a stale or missing binary would debug the wrong
// program, so any build failure surfaces as an error and never launches.
async function runDebug() {
  const doc = activeKarkainDocument();
  if (!doc) { return; }
  const { execFile } = require('child_process');
  const path = require('path');
  const src = doc.uri.fsPath;
  const ext = process.platform === 'win32' ? '.exe' : '';
  const base = path.basename(src, '.kark');
  const program = path.join(path.dirname(src), base + ext);
  const build = await new Promise((resolve) => {
    execFile(compilerPath(), ['build', '-g', src, '-o', program], { encoding: 'utf8' }, (err, stdout, stderr) => {
      resolve({ err, out: String(stdout || '') + String(stderr || '') });
    });
  });
  if (build.err) {
    vscode.window.showErrorMessage('Karkain: debug build failed:\n' + build.out);
    return;
  }
  const folder = vscode.workspace.getWorkspaceFolder(doc.uri);
  const launchConfig = {
    name: 'Karkain: Debug current file (gdb)',
    type: 'cppdbg',
    request: 'launch',
    program,
    args: [],
    stopAtEntry: false,
    cwd: '${workspaceFolder}',
    MIMode: 'gdb',
    miDebuggerPath: debuggerPath(),
    setupCommands: [
      {
        description: 'Pretty-print Karkain Value cells',
        text: '-enable-pretty-printing',
        ignoreFailures: true,
      },
    ],
  };
  const ok = await vscode.debug.startDebugging(folder, launchConfig);
  if (!ok) {
    vscode.window.showErrorMessage('Karkain: debugger did not start. Is the C/C++ extension (cppdbg) installed and gdb on PATH?');
  }
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
  startLanguageClient(context);
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

    vscode.commands.registerCommand('karkain.debug', async () => {
      await runDebug();
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

function deactivate() {
  if (karkainClient) {
    return karkainClient.stop();
  }
}

module.exports = { activate, deactivate };