//go:build linux

package reader

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// processHandle wraps Linux process metadata.
type processHandle struct {
	pid     uint32
	is64bit bool
}

// findProcessByName scans /proc for a process whose Name matches name.
func findProcessByName(name string) (uint32, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("ReadDir /proc: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid64, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil {
			continue
		}
		statusPath := filepath.Join("/proc", e.Name(), "status")
		f, err := os.Open(statusPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Name:") {
				procName := strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
				if procName == name || procName == strings.TrimSuffix(name, ".exe") {
					f.Close()
					return uint32(pid64), nil
				}
				break
			}
		}
		f.Close()
	}
	return 0, fmt.Errorf("process %q not found", name)
}

// openAmongUs opens the Among Us process on Linux.
// On Linux we don't need an explicit "open" — process_vm_readv uses the PID
// directly.  Architecture is detected later (after findModule) via redetectArch.
func openAmongUs() (*processHandle, error) {
	pid, err := findProcessByName("Among Us.exe")
	if err != nil {
		// Try without .exe (Wine / Proton may rename)
		pid, err = findProcessByName("Among Us")
		if err != nil {
			return nil, err
		}
	}
	return &processHandle{pid: pid}, nil
}

// Close is a no-op on Linux (no handle to release).
func (ph *processHandle) Close() {}

// readBytes reads size bytes from address in the remote process using
// process_vm_readv (Linux 3.2+).
func (ph *processHandle) readBytes(address uintptr, size int) ([]byte, error) {
	buf := make([]byte, size)

	localIov := syscall.Iovec{
		Base: &buf[0],
		Len:  uint64(size),
	}
	remoteIov := struct {
		Base uintptr
		Len  uintptr
	}{
		Base: address,
		Len:  uintptr(size),
	}

	n, _, errno := syscall.Syscall6(
		310, // SYS_PROCESS_VM_READV on x86-64
		uintptr(ph.pid),
		uintptr(unsafe.Pointer(&localIov)),
		1,
		uintptr(unsafe.Pointer(&remoteIov)),
		1,
		0,
	)
	if errno != 0 {
		return nil, fmt.Errorf("process_vm_readv(pid=%d, addr=0x%x): %w", ph.pid, address, errno)
	}
	return buf[:n], nil
}

// moduleInfo holds information about a loaded shared library / module.
type moduleInfo struct {
	base uintptr
	size uintptr
	path string
}

// findModule parses /proc/<pid>/maps to locate a mapped module by filename.
//
// Under Wine/Proton only the PE-header page (4 KiB) carries the DLL filename;
// all subsequent sections (.text, .rdata, .data, …) appear as anonymous
// mappings that are contiguous in the address space.  This function therefore:
//  1. Finds the named entry to establish firstBase and lastEnd.
//  2. Continues reading and extends lastEnd for every subsequent anonymous
//     mapping that starts exactly where the previous one ended (contiguous).
//  3. Stops when a gap appears or a non-empty pathname is encountered.
func (ph *processHandle) findModule(name string) (base uintptr, size uint32, err error) {
	mapsPath := fmt.Sprintf("/proc/%d/maps", ph.pid)
	f, e := os.Open(mapsPath)
	if e != nil {
		err = fmt.Errorf("open %s: %w", mapsPath, e)
		return
	}
	defer f.Close()

	var firstBase, lastEnd uintptr
	foundNamed := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: addr_start-addr_end perms offset dev inode [pathname]
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		addrs := strings.SplitN(parts[0], "-", 2)
		if len(addrs) != 2 {
			continue
		}
		start, e1 := strconv.ParseUint(addrs[0], 16, 64)
		end, e2 := strconv.ParseUint(addrs[1], 16, 64)
		if e1 != nil || e2 != nil {
			continue
		}

		// pathname is everything from field index 5 onward (paths may contain
		// spaces, e.g. "Among Us/GameAssembly.dll").
		pathname := ""
		if len(parts) >= 6 {
			pathname = strings.Join(parts[5:], " ")
		}

		if !foundNamed {
			// Looking for the named entry that contains our module name
			if strings.Contains(pathname, name) {
				firstBase = uintptr(start)
				lastEnd = uintptr(end)
				foundNamed = true
			}
			continue
		}

		// We already found the named entry.  Extend through contiguous
		// anonymous mappings (empty pathname) only.
		if pathname != "" {
			// A new named region — stop.
			break
		}
		if uintptr(start) != lastEnd {
			// There is a gap in the address space — stop.
			break
		}
		lastEnd = uintptr(end)
	}

	if firstBase == 0 {
		err = fmt.Errorf("module %q not found in /proc/%d/maps", name, ph.pid)
		return
	}
	return firstBase, uint32(lastEnd - firstBase), nil
}

// redetectArch reads the PE optional-header magic from the GameAssembly.dll
// module base mapped into the Wine/Proton process via process_vm_readv.
// 0x20B = PE32+ (64-bit), 0x10B = PE32 (32-bit).
// Falls back to true (64-bit) on any read error since modern Among Us is 64-bit only.
func (ph *processHandle) redetectArch(modBase uintptr) bool {
	// Read e_lfanew (DOS header offset 0x3C)
	buf, err := ph.readBytes(modBase+0x3c, 4)
	if err != nil {
		return true // assume 64-bit
	}
	lfanew := uintptr(buf[0]) | uintptr(buf[1])<<8 | uintptr(buf[2])<<16 | uintptr(buf[3])<<24
	// PE optional header magic: PE sig (4 bytes) + FileHeader (20 bytes) = +0x18
	magic, err := ph.readBytes(modBase+lfanew+0x18, 2)
	if err != nil {
		return true
	}
	return uint16(magic[0])|uint16(magic[1])<<8 == 0x20b
}

// getProcessPath returns the real path of the process executable.
func (ph *processHandle) getProcessPath() (string, error) {
	link := fmt.Sprintf("/proc/%d/exe", ph.pid)
	target, err := os.Readlink(link)
	if err != nil {
		return "", fmt.Errorf("readlink %s: %w", link, err)
	}
	return target, nil
}
