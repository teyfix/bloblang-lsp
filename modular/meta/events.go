package meta

type HandlerEvent string

const (
	EventSetClient      HandlerEvent = "SetClient"
	EventInitialize     HandlerEvent = "Initialize"
	EventShutdown       HandlerEvent = "Shutdown"
	EventSetTrace       HandlerEvent = "SetTrace"
	EventDidOpen        HandlerEvent = "DidOpen"
	EventDidChange      HandlerEvent = "DidChange"
	EventDidClose       HandlerEvent = "DidClose"
	EventCompletion     HandlerEvent = "Completion"
	EventHover          HandlerEvent = "Hover"
	EventInlayHint      HandlerEvent = "InlayHint"
	EventCodeLens       HandlerEvent = "CodeLens"
	EventExecuteCommand HandlerEvent = "ExecuteCommand"
)
