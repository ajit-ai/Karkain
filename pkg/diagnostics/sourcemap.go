package diagnostics

import "encoding/json"

// MappingSegment tracks a single source location mapping
type MappingSegment struct {
	GeneratedLine  int    `json:"generatedLine"`
	GeneratedColumn int   `json:"generatedColumn"`
	OriginalLine   int    `json:"originalLine"`
	OriginalColumn  int    `json:"originalColumn"`
	SymbolName     string `json:"symbolName,omitempty"`
}

// SourceMap tracks mappings between generated output and original source
type SourceMap struct {
	SourceFile string           `json:"sourceFile"`
	Mappings   []MappingSegment `json:"mappings"`
}

// NewSourceMap creates a new source map for a given source file
func NewSourceMap(sourceFile string) *SourceMap {
	return &SourceMap{
		SourceFile: sourceFile,
		Mappings:   []MappingSegment{},
	}
}

// AddMapping records a mapping from generated output back to original source
func (sm *SourceMap) AddMapping(genLine, genCol, origLine, origCol int, symbol string) {
	sm.Mappings = append(sm.Mappings, MappingSegment{
		GeneratedLine:    genLine,
		GeneratedColumn:  genCol,
		OriginalLine:     origLine,
		OriginalColumn:   origCol,
		SymbolName:       symbol,
	})
}

// ToJSON serializes the source map to a JSON byte slice
func (sm *SourceMap) ToJSON() ([]byte, error) {
	return json.Marshal(sm)
}
