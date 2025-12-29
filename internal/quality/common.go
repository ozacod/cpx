package quality

// VcpkgSetup defines the interface for vcpkg operations needed by quality tools
type VcpkgSetup interface {
	SetupEnv() error
	GetPath() (string, error)
}
