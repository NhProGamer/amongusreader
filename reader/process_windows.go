//go:build windows

package reader

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processHandle wraps the Win32 handle and metadata for the Among Us process.
type processHandle struct {
	handle  windows.Handle
	pid     uint32
	is64bit bool
}

var (
	modKernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procReadProcessMemory  = modKernel32.NewProc("ReadProcessMemory")
	procVirtualAllocEx     = modKernel32.NewProc("VirtualAllocEx")
	procWriteProcessMemory = modKernel32.NewProc("WriteProcessMemory")

	modPsapi                 = windows.NewLazySystemDLL("psapi.dll")
	procEnumProcessModulesEx = modPsapi.NewProc("EnumProcessModulesEx")
	procGetModuleBaseNameW   = modPsapi.NewProc("GetModuleBaseNameW")
	procGetModuleInformation = modPsapi.NewProc("GetModuleInformation")
)

// MODULEINFO mirrors the Win32 MODULEINFO struct.
type MODULEINFO struct {
	BaseOfDll   uintptr
	SizeOfImage uint32
	EntryPoint  uintptr
}

// findProcessByName returns the PID of the first process whose executable
// filename matches name (e.g. "Among Us.exe").
func findProcessByName(name string) (uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))

	if err := windows.Process32First(snapshot, &pe); err != nil {
		return 0, fmt.Errorf("Process32First: %w", err)
	}
	for {
		if windows.UTF16ToString(pe.ExeFile[:]) == name {
			return pe.ProcessID, nil
		}
		if err := windows.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}
	return 0, fmt.Errorf("process %q not found", name)
}

// openAmongUs opens the Among Us process with VM_READ rights and returns
// a processHandle ready for memory operations.
func openAmongUs() (*processHandle, error) {
	pid, err := findProcessByName("Among Us.exe")
	if err != nil {
		return nil, err
	}

	const access = windows.PROCESS_VM_READ |
		windows.PROCESS_QUERY_INFORMATION |
		windows.PROCESS_VM_WRITE | // needed for shellcode (future use)
		windows.PROCESS_VM_OPERATION

	h, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		return nil, fmt.Errorf("OpenProcess(pid=%d): %w", pid, err)
	}

	ph := &processHandle{handle: h, pid: pid}
	ph.is64bit = detectIs64Bit(h)
	return ph, nil
}

// Close releases the Win32 process handle.
func (ph *processHandle) Close() {
	if ph.handle != 0 {
		windows.CloseHandle(ph.handle)
		ph.handle = 0
	}
}

// readBytes reads size bytes from address in the remote process.
func (ph *processHandle) readBytes(address uintptr, size int) ([]byte, error) {
	buf := make([]byte, size)
	var nRead uintptr
	r, _, err := procReadProcessMemory.Call(
		uintptr(ph.handle),
		address,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&nRead)),
	)
	if r == 0 {
		return nil, fmt.Errorf("ReadProcessMemory(addr=0x%x, size=%d): %w", address, size, err)
	}
	return buf[:nRead], nil
}

// findModule returns the base address and size of a named module loaded in the
// remote process (e.g. "GameAssembly.dll").
func (ph *processHandle) findModule(name string) (base uintptr, size uint32, err error) {
	// Enumerate modules — use LIST_MODULES_ALL to cover 32-bit and 64-bit.
	const LIST_MODULES_ALL = 0x03
	var hMods [1024]windows.Handle
	var needed uint32

	r, _, e := procEnumProcessModulesEx.Call(
		uintptr(ph.handle),
		uintptr(unsafe.Pointer(&hMods[0])),
		uintptr(uint32(unsafe.Sizeof(hMods))),
		uintptr(unsafe.Pointer(&needed)),
		LIST_MODULES_ALL,
	)
	if r == 0 {
		err = fmt.Errorf("EnumProcessModulesEx: %w", e)
		return
	}

	count := needed / uint32(unsafe.Sizeof(hMods[0]))
	nameBuf := make([]uint16, 260)

	for i := uint32(0); i < count; i++ {
		r, _, _ := procGetModuleBaseNameW.Call(
			uintptr(ph.handle),
			uintptr(hMods[i]),
			uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(len(nameBuf)),
		)
		if r == 0 {
			continue
		}
		if windows.UTF16ToString(nameBuf[:r]) == name {
			var info MODULEINFO
			r2, _, e2 := procGetModuleInformation.Call(
				uintptr(ph.handle),
				uintptr(hMods[i]),
				uintptr(unsafe.Pointer(&info)),
				uintptr(unsafe.Sizeof(info)),
			)
			if r2 == 0 {
				err = fmt.Errorf("GetModuleInformation(%s): %w", name, e2)
				return
			}
			return info.BaseOfDll, info.SizeOfImage, nil
		}
	}
	err = fmt.Errorf("module %q not found in process %d", name, ph.pid)
	return
}

// detectIs64Bit reads the PE optional-header magic to determine architecture.
// 0x20B = PE32+ (64-bit), 0x10B = PE32 (32-bit).
func detectIs64Bit(h windows.Handle) bool {
	// Read e_lfanew from DOS header
	lfanewBuf := make([]byte, 4)
	var nRead uintptr
	// We need the module base first — we'll call this after findModule sets base.
	// As a bootstrap we do a tiny read of the module base inside openAmongUs.
	// This function is called right after openProcess, before findModule,
	// so we read it from GameAssembly once available. For now return false;
	// the Reader will call redetect after findModule.
	_ = h
	_ = lfanewBuf
	_ = nRead
	return false
}

// redetectArch checks the PE header of modBase to determine 32/64 bit.
func (ph *processHandle) redetectArch(modBase uintptr) bool {
	// offset 0x3C = e_lfanew (RVA of PE signature)
	buf, err := ph.readBytes(modBase+0x3c, 4)
	if err != nil {
		return false
	}
	lfanew := uintptr(buf[0]) | uintptr(buf[1])<<8 | uintptr(buf[2])<<16 | uintptr(buf[3])<<24
	// PE optional header magic is at PE_signature(4) + FileHeader(20) + Magic(2)
	// = e_lfanew + 0x18
	magic, err := ph.readBytes(modBase+lfanew+0x18, 2)
	if err != nil {
		return false
	}
	return uint16(magic[0])|uint16(magic[1])<<8 == 0x20b
}

// getProcessPath returns the full path of the Among Us executable.
func (ph *processHandle) getProcessPath() (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(ph.handle, 0, &buf[0], &size); err != nil {
		return "", fmt.Errorf("QueryFullProcessImageName: %w", err)
	}
	return windows.UTF16ToString(buf[:size]), nil
}
