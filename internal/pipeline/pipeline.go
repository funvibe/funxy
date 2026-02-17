package pipeline

// Pipeline represents a sequence of processing stages.
type Pipeline struct {
	processors []Processor
}

func New(processors ...Processor) *Pipeline {
	return &Pipeline{processors: processors}
}

// Run executes the pipeline.
func (p *Pipeline) Run(initialCtx *PipelineContext) *PipelineContext {
	ctx := initialCtx
	for _, processor := range p.processors {
		ctx = processor.Process(ctx)
		// Continue on errors to collect diagnostics from all stages
		// (e.g. LSP needs both parse and semantic errors).
	}
	return ctx
}
