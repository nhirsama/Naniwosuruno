//go:build !windows

package Windows

import (
	"errors"

	"github.com/nhirsama/Naniwosuruno/client/inter"
)

type WindowTitle struct {
}

// NewWindowTitle 是一个存根，用于在非 Windows 系统下满足编译要求。
// 实际运行时，client.go 中的 switch c.os 逻辑会防止此函数被调用。
func NewWindowTitle() inter.GetWindowTitle {
	return &WindowTitle{}
}

func (w *WindowTitle) GetWindowTitle() (string, error) {
	return "", errors.New("windows implementation is not available on this platform")
}
