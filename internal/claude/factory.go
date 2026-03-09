package claude

import "go.uber.org/zap"

// Factory creates Claude client instances.
type Factory struct {
	logger *zap.Logger
}

// NewFactory creates a new Factory.
func NewFactory(logger *zap.Logger) *Factory {
	return &Factory{logger: logger}
}

// NewCLI returns a Client using the default "claude" binary from PATH.
func (f *Factory) NewCLI() Client {
	return NewCLIClient(f.logger)
}

// NewCLIWithPath returns a Client using a specific binary path.
func (f *Factory) NewCLIWithPath(path string) Client {
	return NewCLIClientWithPath(path, f.logger)
}
