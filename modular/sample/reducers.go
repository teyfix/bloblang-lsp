package sample

import (
	"github.com/teyfix/bloblang-lsp/modular/pipeline"
)

// 1. Sample Reducer
type SampleReducer struct {
	manager *Manager
}

func NewSampleReducer(mgr *Manager) *SampleReducer {
	return &SampleReducer{
		manager: mgr,
	}
}

func (r *SampleReducer) Name() string { return "sample" }

func (r *SampleReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventDidOpen, pipeline.EventDidChange, pipeline.EventDidClose}
}

func (r *SampleReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	if change.Kind == pipeline.EventDidClose {
		r.manager.Remove(string(state.URI))
		return &pipeline.EventResult{}
	}
	diags := r.manager.Update(string(state.URI), state.Text, state.Tree, state.BaseDir)
	return &pipeline.EventResult{
		ListDiagnostic: diags,
	}
}

// 2. Inlay Hints Reducer
type InlayHintsReducer struct {
	manager *Manager
}

func NewInlayHintsReducer(mgr *Manager) *InlayHintsReducer {
	return &InlayHintsReducer{
		manager: mgr,
	}
}

func (r *InlayHintsReducer) Name() string { return "inlay_hints" }

func (r *InlayHintsReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventInlayHint}
}

func (r *InlayHintsReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	hints, diags := r.manager.ComputeInlayHints(string(state.URI), state.Text, state.Tree, state.BaseDir)
	return &pipeline.EventResult{
		ListInlayHint:  hints,
		ListDiagnostic: diags,
	}
}

// 3. Code Lenses Reducer
type CodeLensesReducer struct {
	manager *Manager
}

func NewCodeLensesReducer(mgr *Manager) *CodeLensesReducer {
	return &CodeLensesReducer{
		manager: mgr,
	}
}

func (r *CodeLensesReducer) Name() string { return "code_lenses" }

func (r *CodeLensesReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventCodeLens}
}

func (r *CodeLensesReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	lenses, diags := r.manager.ComputeCodeLenses(string(state.URI), state.Text, state.Tree, state.BaseDir)
	return &pipeline.EventResult{
		ListCodeLens:   lenses,
		ListDiagnostic: diags,
	}
}
