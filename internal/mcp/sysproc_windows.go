//go:build windows

package mcp

import "syscall"

// detachedProcess is the Win32 CreationFlag that fully detaches a child from
// the parent's console. Not exported by Go's syscall package on all versions,
// so we declare it locally — value matches the Win32 SDK constant.
const detachedProcess = 0x00000008

func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
}

// processAlive checks whether a process with the given pid is currently
// running. Uses OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION (0x1000)
// so it works for processes owned by the current user without requiring
// elevation.
func processAlive(pid int) bool {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}
