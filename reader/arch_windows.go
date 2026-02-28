//go:build windows

package reader

// redetectArchWithBase wraps the Windows PE-based architecture detection,
// providing the same call signature as the Linux version.
func (ph *processHandle) redetectArchWithBase(modBase uintptr) bool {
	return ph.redetectArch(modBase)
}
