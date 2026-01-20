package LinuxKDE

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nhirsama/Naniwosuruno/client/inter"
	"github.com/nhirsama/Naniwosuruno/pkg"
)

type WindowTitle struct {
}

func NewWindowTitle() inter.GetWindowTitle {
	return &WindowTitle{}
}
func (w *WindowTitle) getRawWindowTitle() (string, error) {
	cmd := exec.Command("./kdotool", "getactivewindow", "getwindowname")
	out, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return "", errors.New(fmt.Sprintf("获取窗口标题出错，程序退出码为：%d。%s\n", exitError.ExitCode(), exitError.Stderr))
		}
	}
	outStr := strings.TrimSpace(string(out))
	return outStr, nil
}

func (w *WindowTitle) GetWindowTitle() (string, error) {
	windowRawTitle, err := w.getRawWindowTitle()
	if err != nil {
		return "", err
	}
	return pkg.CleanWindowTitle(windowRawTitle), nil
}
