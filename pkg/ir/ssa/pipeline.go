package ssa

// Pipeline — Phase 94: multi-pass SSA optimization pipeline.
//
// Passes run in sequence: mem2reg → constant fold → CSE → DCE → LICM.
// The pipeline iterates to fixpoint (max 10 iterations).

// Pass is a single optimization pass that transforms a function.
type Pass interface {
	Name() string
	Run(fn *Function) bool
}

// Pipeline chains multiple optimization passes.
type Pipeline struct {
	passes []Pass
}

// NewPipeline creates a pipeline with the default optimization order.
func NewPipeline() *Pipeline {
	return &Pipeline{
		passes: []Pass{
			&Mem2RegPass{},
			&FoldConstPass{},
			&CSEPass{},
			&DCESimplePass{},
			&LICMPass{},
			&SAROPass{},
		},
	}
}

// Add appends a pass to the pipeline.
func (p *Pipeline) Add(pass Pass) {
	p.passes = append(p.passes, pass)
}

// Run executes all passes on a function until fixpoint or max iterations.
// Returns the total number of passes that made changes across all iterations.
func (p *Pipeline) Run(fn *Function) int {
	total := 0
	const maxIters = 10
	for iter := 0; iter < maxIters; iter++ {
		iterChanged := false
		for _, pass := range p.passes {
			if pass.Run(fn) {
				total++
				iterChanged = true
			}
		}
		if !iterChanged {
			break
		}
	}
	return total
}

// RunModule runs the pipeline on every function in a module.
func (p *Pipeline) RunModule(m *Module) int {
	total := 0
	for _, fn := range m.Functions {
		total += p.Run(fn)
	}
	return total
}

// RunWithStats executes the pipeline once and records per-pass statistics.
func (p *Pipeline) RunWithStats(fn *Function) []PassStats {
	var stats []PassStats
	for _, pass := range p.passes {
		stats = append(stats, PassStats{
			Name:    pass.Name(),
			Changed: pass.Run(fn),
		})
	}
	return stats
}

// PassStats records whether a single pass made changes.
type PassStats struct {
	Name    string
	Changed bool
}
