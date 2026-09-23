package app

import (
	"context"
	"log"
	"net"
	"os"
	"strings"

	"singbox-web/internal/core"
)

// Core and Web messages share one logger so concurrent writes cannot interleave.
type coreConsoleWriter struct{ console *log.Logger }

func (w coreConsoleWriter) Write(payload []byte) (int, error) {
	err := w.console.Output(2, "[核心] "+strings.TrimSuffix(string(payload), "\n"))
	return len(payload), err
}

func (s *Server) logInfo(format string, args ...any) {
	if s.console != nil {
		s.console.Printf("[Web] "+format, args...)
	}
}

// Print the bound address only after Listen succeeds, including an OS-assigned
// port when an isolated caller requests port zero.
func (s *Server) printStartup(address net.Addr) {
	s.logInfo("服务已启动，PID：%d", os.Getpid())
	s.logInfo("监听地址：http://%s", address.String())
	host, port, err := net.SplitHostPort(address.String())
	if err == nil {
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			host = "127.0.0.1"
			if ip.To4() == nil {
				host = "::1"
			}
		}
		s.logInfo("本机访问：http://%s", net.JoinHostPort(host, port))
	}
	s.logInfo("工作目录：%s", s.layout.WorkDir)
	s.logInfo("配置目录：%s", s.layout.ConfigDir)
	s.logInfo("默认 Mixed 入站端口：%d（用于新建入站，实际监听以核心日志为准）", s.defaultMixedPort)
	s.logInfo("核心日志：%s", s.layout.CoreLog)
	if s.core == nil {
		return
	}
	status := s.core.Status(context.Background())
	state := map[core.ProcessState]string{
		core.ProcessNotInstalled: "未安装",
		core.ProcessStopped:      "已停止",
		core.ProcessStarting:     "正在启动",
		core.ProcessRunning:      "正在运行",
		core.ProcessStopping:     "正在停止",
		core.ProcessStartFailed:  "启动失败",
		core.ProcessExited:       "已退出",
	}[status.State]
	if state == "" {
		state = string(status.State)
	}
	s.logInfo("核心状态：%s", state)
	if status.InstalledVersion != "" {
		s.logInfo("核心版本：%s", status.InstalledVersion)
	}
	if status.PID > 0 {
		s.logInfo("核心 PID：%d", status.PID)
	}
	if status.LastError != "" {
		s.logInfo("核心错误：%s", status.LastError)
	}
}
