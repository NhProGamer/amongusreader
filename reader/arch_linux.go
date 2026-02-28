//go:build linux

package reader

// redetectArchWithBase delegates to the PE-based detection in process_linux.go,
// which reads the PE header directly from the GameAssembly.dll module memory.
func (ph *processHandle) redetectArchWithBase(modBase uintptr) bool {
	return ph.redetectArch(modBase)
}
