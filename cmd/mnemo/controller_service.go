package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

const controllerLaunchLabel = "com.jmeiracorbal.mnemo.controller"

// installControllerService registers the installation-owned controller with the
// OS service manager. It is deliberately never called from MCP startup.
func installControllerService(home, mnemoBin string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("persistent controller service is not yet supported on %s", runtime.GOOS)
	}
	path := filepath.Join(home, "Library", "LaunchAgents", controllerLaunchLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	content := controllerLaunchAgentPlist(home, mnemoBin)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, path).Run()
	if output, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return "", fmt.Errorf("bootstrap controller service: %w: %s", err, output)
	}
	return path, nil
}

func controllerLaunchAgentPlist(home, mnemoBin string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>controller</string><string>serve</string></array>
<key>EnvironmentVariables</key><dict><key>HOME</key><string>%s</string></dict>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
<key>StandardOutPath</key><string>%s</string><key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, controllerLaunchLabel, mnemoBin, home, filepath.Join(home, ".mnemo", "controller.log"), filepath.Join(home, ".mnemo", "controller.err.log"))
}
