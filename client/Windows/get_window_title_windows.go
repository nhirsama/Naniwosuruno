//go:build windows

package Windows

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nhirsama/Naniwosuruno/client/inter"
	"github.com/nhirsama/Naniwosuruno/pkg"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procGetModuleBaseNameW       = psapi.NewProc("GetModuleBaseNameW")
)

const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
)

type WindowTitle struct {
}

func NewWindowTitle() inter.GetWindowTitle {
	return &WindowTitle{}
}

func (w *WindowTitle) getRawWindowTitle() (string, error) {
	// 获取当前焦点窗口句柄
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", nil // 无焦点窗口
	}

	// 获取窗口对应的进程 ID (PID)
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	// 打开进程
	// PROCESS_QUERY_INFORMATION | PROCESS_VM_READ
	hProcess, _, err := procOpenProcess.Call(uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ), 0, uintptr(pid))
	if hProcess == 0 {
		return "", fmt.Errorf("open process failed: %v", err)
	}
	defer procCloseHandle.Call(hProcess)

	// 获取进程名称
	buf := make([]uint16, 1024)
	ret, _, err := procGetModuleBaseNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return "", fmt.Errorf("get module base name failed: %v", err)
	}

	// 转换 UTF-16 到 string
	name := syscall.UTF16ToString(buf)

	// 去除路径和扩展名（虽然 GetModuleBaseName 通常只返回名字，但为了保险起见处理一下）
	name = filepath.Base(name)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	return name, nil
}

func (w *WindowTitle) GetWindowTitle() (string, error) {
	windowRawTitle, err := w.getRawWindowTitle()
	if err != nil {
		return "", err
	}
	// 保持与 Linux 实现一致的格式化逻辑
	return pkg.FormatAppClass(windowRawTitle), nil
}
