# Native Codegen, Linker & Debug Information

Phase 84: the foundation for native object representation, linking, symbol
handling, relocation processing, and source-level debug mapping in Karkain.

---

## 1. Pipeline Overview

```
Karkain source (.kark)
        |
    Parser / Semantic Analysis
        |
    AST (parser.Program)
        |
   +----+----+
   |         |
Native     GenerateObject
Generator   (sections, symbols,
(C11 code,  debug info)
 LLVM IR)
   |         |
   v         v
C source   Object representation
           (pkg/codegen)
                |
            Linker
                |
            Executable
```

`NativeGenerator.GenerateModule` produces C11/LLVM IR source text.  
`NativeGenerator.GenerateObject` builds a Karkain-owned native object
from the AST: sections, symbols (with `karkain_user_*` namespacing), and
debug information (line mappings, function/variable info).

Both paths share the Phase-83 diagnostic framework.

---

## 2. Object Model (`object.go`)

The central representation for native object output:

```
Object
  Name        string
  Sections    []*Section
  Symbols     []*Symbol
  Relocations []*Relocation
  DebugInfo   *DebugInfo
```

### Section

| field   | meaning                                 |
|---------|-----------------------------------------|
| Name    | section name (`.text`, `.data`, etc.)   |
| Type    | SectionType enum                        |
| Data    | raw bytes                               |
| Address | virtual address (set by linker)         |
| Align   | alignment requirement                   |
| Size    | size in bytes                           |

Section types: `SectionTypeText`, `SectionTypeData`, `SectionTypeBSS`,
`SectionTypeROData`, `SectionTypeDebug`.

---

## 3. Symbols (`symbols.go`)

### Symbol Table

`SymbolTable` provides scope-aware symbol management:

```go
st := NewSymbolTable()
st.DefineSymbol(name, type, binding, section, value, size)
st.LookupSymbol(name)  // searches current scope outward
st.EnterScope("fn")
st.ExitScope()
```

### Symbol Attributes

| field    | meaning                                     |
|----------|---------------------------------------------|
| Name     | mangled name (`karkain_user_*` for user fns)|
| Type     | `Function`, `Object`, `Section`, `File`     |
| Binding  | `Local`, `Global`, `Weak`                   |
| Section  | owning section name                         |
| Value    | offset within section                       |
| Size     | size in bytes                               |

### Namespacing

User-defined symbols are automatically namespaced via `userFuncC()` to
`karkain_user_*`, preventing collisions with C runtime symbols. Special
symbols (`main`, `getArgs`) keep canonical names.

`CollectSymbolsFromAST` walks the AST and populates both the SymbolTable
and an Object's symbol list, respecting namespacing rules.

`ValidateSymbolNames` rejects symbols that collide with `karkain_` or
`__karkain` runtime prefixes.

---

## 4. Relocations (`relocation.go`)

`RelocationManager` tracks and applies address fixups:

```go
rm := NewRelocationManager()
rm.RegisterSymbol(sym)
rm.AddRelocation(offset, relocType, symbol, addend, section)
rm.ApplyRelocations(section)  // patches section bytes in-place
rm.ValidateRelocations()      // checks all symbols are defined
```

### Relocation Types

| Type        | Width | Description                       |
|-------------|-------|-----------------------------------|
| `Addr32`    | 4     | 32-bit absolute address           |
| `Addr64`    | 8     | 64-bit absolute address           |
| `PCRel32`   | 4     | 32-bit PC-relative (call/jmp)     |
| `PCRel64`   | 8     | 64-bit PC-relative                |
| `PLT32`     | 4     | PLT-relative (link-time)          |
| `GOT32`     | 4     | GOT-relative (link-time)          |

Convenience constructors:
- `RelocationForFunctionCall(offset, name, section)` — creates PCRel32
- `RelocationForGlobalVariable(offset, name, section)` — creates Addr64

---

## 5. Linker (`linker.go`)

`Linker` transforms one or more Object files into an Executable:

```go
l := NewLinker()
l.SetEntryPoint("main")
l.addObject(obj)
exec, err := l.Link()
```

### Link Process

1. **Symbol collection** — gather all symbols from objects; reject
   duplicate globals
2. **Symbol resolution** — verify entry point exists; verify all
   relocated symbols are defined
3. **Section layout** — merge same-named sections; assign virtual
   addresses starting at `0x1000` with proper alignment
4. **Relocation application** — patch section data bytes
5. **Entry point resolution** — locate the entry symbol's final address
6. **Executable assembly** — produce the final `Executable`

### Executable

```go
type Executable struct {
    Sections      []*Section
    EntryPoint    uint64
    Symbols       map[string]*Symbol
    EntryPointName string
    DebugInfo     *DebugInfo
}
```

Methods: `GetSection(name)`, `GetSymbol(name)`, `Size()`, `Validate()`.

`MinimalLinker` wraps `Linker` with sensible defaults for single-object
or multi-object programs with `main` as entry.

---

## 6. Debug Information (`debug.go`)

### DebugInfoBuilder

Collects source-level debug information from an AST:

```go
dib := NewDebugInfoBuilder()
dib.CollectDebugInfoFromAST(prog, "main.kark")
dbg := dib.GetDebugInfo()
```

Produces:

- **SourceFile** entries (path, directory, optional checksum)
- **LineInfo** entries (address, file index, line, column, span length,
  containing function)
- **FunctionInfo** entries (name, address, size, file index, start/end
  lines, parameters)
- **VariableInfo** entries (name, type, stack address, file index, line,
  local flag, containing function)

### SourceAddressMap

Bidirectional mapping between source locations and native addresses:

```go
sam := NewSourceAddressMap()
sam.AddMapping("main.kark", 12, 0x401024, "main")
addr, _ := sam.GetAddressForSource("main.kark", 12)  // → 0x401024
loc, _  := sam.GetSourceForAddress(0x401024)         // → {main.kark, 12, main}
```

`BuildFromDebugInfo` populates the map from a `DebugInfo`'s `LineInfo`
entries, enabling:

- **Forward**: source location → native address (for breakpoints)
- **Reverse**: native address → source location (for stack traces)

---

## 7. Error Handling

All native codegen and linker failures use the Phase-83 diagnostic
framework (`E-K-CG` code). Report functions:

| Function                         | Severity | Use case                    |
|----------------------------------|----------|-----------------------------|
| `ReportNativeCodegenError`       | error    | code generation failure     |
| `ReportSymbolDiagnostic`         | error    | symbol collision/undefined  |
| `ReportRelocationDiagnostic`     | error    | relocation failure          |
| `ReportLinkerDiagnostic`         | error    | linker stage failure        |
| `ReportObjectDiagnostic`         | error    | object file generation      |
| `ReportDebugDiagnostic`          | warning  | debug info incomplete       |

`CodegenDiagnostics` wraps the Phase-83 diagnostic type with convenience
methods (`AddError`, `AddErrorWithHelp`, `ReportSymbolError`, etc.) and
counting helpers (`HasErrors`, `HasWarnings`, `ErrorCount`).

---

## 8. Integration

### `NativeGenerator.GenerateObject`

The bridge from compiler structures to native object:

```go
gen := NewNativeGenerator()
obj, diags, err := gen.GenerateObject(prog, "main.kark")
```

This method:
1. Creates an Object with `.text`, `.rodata`, `.data` sections
2. Collects symbols via `SymbolTable.CollectSymbolsFromAST` (applies
   `karkain_user_*` namespacing)
3. Validates symbol names against runtime prefixes
4. Collects debug information via `DebugInfoBuilder`
5. Returns the Object, any diagnostics, and an error

The returned Object is ready for the Linker and can carry relocation
entries and debug info for future backend emission passes.

---

## 9. CLI Integration (Phase 85)

### Build Flow

The `--target=native-link` flag integrates Phase-84 infrastructure into the
normal Karkain CLI compilation workflow:

```
program.kark
    |
CLI (--target=native-link)
    |
Parser → Semantic Analysis
    |
NativeGenerator.GenerateObject
    |  (sections, symbols, debug info)
    v
Object
    |
Linker (symbol resolution, section layout, relocations)
    |
Executable (in-memory)
    |
NativeBuilder.Build produces NativeBuildResult
```

### Usage

```bash
karkain build --target=native-link program.kark
karkain run  --target=native-link program.kark
```

### NativeBuilder (`native_builder.go`)

Orchestrates the integrated pipeline:

```go
builder := NewNativeBuilder()
result, err := builder.Build(prog, "program.kark")
// result.Object      — native object (sections, symbols, debug info)
// result.Executable  — linked executable (sections, entry point)
// result.DebugInfo   — source-level debug information
// result.SourceMap   — bidirectional source↔address mapping
// result.Diagnostics — structured diagnostics
```

`Build` performs:
1. `NativeGenerator.GenerateObject` → Object with sections, symbols, debug info
2. `Linker.AddObject` + `Link` → Executable with resolved symbols and layout
3. `relocateDebugAddresses` → offsets debug info addresses to match the
   linked .text section base (0x1000+), ensuring source-to-address mappings
   reflect actual virtual addresses
4. `SourceAddressMap.BuildFromDebugInfo` → bidirectional source↔address map

### BuildMultiObject

Links multiple Object files into a single Executable:

```go
builder := NewNativeBuilder()
result, err := builder.BuildMultiObject(objects, "main")
```

### CLI Dispatch

`BuildCommand` and `RunCommand` dispatch to `nativeBuildCommand` /
`nativeRunCommand` when `cfg.Target == "native-link"`. The native path:

- Parses source → AST
- Runs `NativeBuilder.Build` (Object → Linker → Executable)
- Formats diagnostics via `formatDiagnostics`
- Writes a KOBJ artifact (binary format with sections/symbols)
- Reports results (verbose mode prints object/executable stats)

### Artifact Format (KOBJ)

The CLI writes KOBJ binary artifacts:

```
Magic:     "KOBJ"
Sections:  name, type, alignment, data bytes
Symbols:   name, type, binding, section index, value, size
```

### Debug Information Preservation

After linking, debug addresses are relocated from 0-based counters to
actual virtual addresses (offset by .text section base). This ensures:

- Source line → address mappings match the linked executable
- Address → source line reverse lookups work for stack traces
- Function and variable info reflects actual runtime addresses
