//go:build windows

package Windows

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nhirsama/Naniwosuruno/internal/client/inter"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	version  = syscall.NewLazyDLL("version.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")

	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")

	procGetFileVersionInfoSizeW = version.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = version.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = version.NewProc("VerQueryValueW")

	// Windows 11 获取友好名称的核心 API
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
	procCoTaskMemFree               = ole32.NewProc("CoTaskMemFree")
)

const (
	ProcessQueryLimitedInformation = 0x1000
	SigdnNormaldisplay             = 0 // 获取常规显示名称
)

// IidIshellitem 接口标识符 (GUID)
var IidIshellitem = syscall.GUID{
	Data1: 0x43826d1e,
	Data2: 0xe718,
	Data3: 0x42ee,
	Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe},
}

type WindowTitle struct {
}

func NewWindowTitle() inter.GetWindowTitle {
	return &WindowTitle{}
}

func (w *WindowTitle) getRawWindowTitle() (string, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", nil
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	hProcess, _, _ := procOpenProcess.Call(uintptr(ProcessQueryLimitedInformation), 0, uintptr(pid))
	if hProcess == 0 {
		return "", fmt.Errorf("open process failed")
	}
	defer procCloseHandle.Call(hProcess)

	exePathBuf := make([]uint16, 1024)
	exePathLen := uint32(len(exePathBuf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&exePathBuf[0])), uintptr(unsafe.Pointer(&exePathLen)))
	if ret == 0 {
		return "", fmt.Errorf("query process name failed")
	}
	exePath := syscall.UTF16ToString(exePathBuf)

	// 优先使用 Shell Item 获取 Windows 11 友好名称 ---
	shellName := w.getFriendlyNameViaShell(exePath)
	if shellName != "" {
		return shellName, nil
	}

	// 尝试从版本信息获取
	friendlyName := w.extractFileDescription(exePath)
	if friendlyName != "" {
		return friendlyName, nil
	}

	// 文件名
	name := filepath.Base(exePath)
	return strings.TrimSuffix(name, filepath.Ext(name)), nil
}

func (w *WindowTitle) GetWindowTitle() (string, error) {
	windowRawTitle, err := w.getRawWindowTitle()
	if err != nil {
		return "", err
	}
	return windowRawTitle, nil
}

// Windows 11 通过 Shell 命名空间获取本地化名称
func (w *WindowTitle) getFriendlyNameViaShell(path string) string {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	var shellItem uintptr

	// 创建 IShellItem 对象
	ret, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&IidIshellitem)),
		uintptr(unsafe.Pointer(&shellItem)),
	)

	if ret != 0 { // S_OK = 0
		return ""
	}
	defer func() {
		// 释放 IShellItem (调用 IUnknown::Release)
		if shellItem != 0 {
			// vtable 索引 2 是 Release
			ptr := (*uintptr)(unsafe.Pointer(shellItem))
			syscall.SyscallN(*(*uintptr)(unsafe.Pointer(*ptr + 2*unsafe.Sizeof(uintptr(0)))), shellItem)
		}
	}()

	// 调用 IShellItem::GetDisplayName (vtable 索引为 5)
	var displayNamePtr uintptr
	ptr := (*uintptr)(unsafe.Pointer(shellItem))
	getDisplayNameAddr := *(*uintptr)(unsafe.Pointer(*ptr + 5*unsafe.Sizeof(uintptr(0))))

	ret, _, _ = syscall.SyscallN(getDisplayNameAddr, shellItem, uintptr(SigdnNormaldisplay), uintptr(unsafe.Pointer(&displayNamePtr)))

	if ret == 0 && displayNamePtr != 0 {
		name := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(displayNamePtr))[:])
		procCoTaskMemFree.Call(displayNamePtr) // 必须释放内存
		return name
	}

	return ""
}

// (保持 extractFileDescription 不变作为普通应用的兼容性补充)
func (w *WindowTitle) extractFileDescription(path string) string {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	var handle uint32
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&handle)))
	if size == 0 {
		return ""
	}
	data := make([]byte, size)
	ret, _, _ := procGetFileVersionInfoW.Call(uintptr(unsafe.Pointer(pathPtr)), 0, size, uintptr(unsafe.Pointer(&data[0])))
	if ret == 0 {
		return ""
	}
	var subBlock *uint32
	var subBlockLen uint32
	langPtr, _ := syscall.UTF16PtrFromString("\\VarFileInfo\\Translation")
	procVerQueryValueW.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(langPtr)), uintptr(unsafe.Pointer(&subBlock)), uintptr(unsafe.Pointer(&subBlockLen)))
	subBlockPath := "\\StringFileInfo\\040904b0\\FileDescription"
	if subBlockLen > 0 {
		langID := *subBlock
		subBlockPath = fmt.Sprintf("\\StringFileInfo\\%04x%04x\\FileDescription", langID&0xffff, langID>>16)
	}
	var descPtr *uint16
	var descLen uint32
	descPathPtr, _ := syscall.UTF16PtrFromString(subBlockPath)
	procVerQueryValueW.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(descPathPtr)), uintptr(unsafe.Pointer(&descPtr)), uintptr(unsafe.Pointer(&descLen)))
	if descLen > 0 {
		return syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(descPtr))[:descLen])
	}
	return ""
}
