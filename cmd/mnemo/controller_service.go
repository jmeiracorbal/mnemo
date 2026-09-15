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
const controllerSystemdName = "mnemo-controller.service"
const controllerWindowsTask = "MnemoController"

// installControllerService registers the installation-owned controller with
// the native user-level service manager. It is never called from MCP startup.
func installControllerService(home, mnemoBin string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchdController(home, mnemoBin)
	case "linux":
		return installSystemdController(home, mnemoBin)
	case "windows":
		return installWindowsController(home, mnemoBin)
	default:
		return "", fmt.Errorf("persistent controller service is not supported on %s", runtime.GOOS)
	}
}

func installLaunchdController(home, mnemoBin string) (string, error) {
	path := filepath.Join(home, "Library", "LaunchAgents", controllerLaunchLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(controllerLaunchAgentPlist(home, mnemoBin)), 0644); err != nil {
		return "", err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, path).Run()
	if out, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return "", fmt.Errorf("bootstrap controller service: %w: %s", err, out)
	}
	return path, nil
}

func installSystemdController(home, mnemoBin string) (string, error) {
	path := filepath.Join(home, ".config", "systemd", "user", controllerSystemdName)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(controllerSystemdUnit(home, mnemoBin)), 0644); err != nil {
		return "", err
	}
	for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "--now", controllerSystemdName}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("systemd controller service: %w: %s", err, out)
		}
	}
	return path, nil
}

func installWindowsController(home, mnemoBin string) (string, error) {
	command := fmt.Sprintf(`"%s" controller serve`, mnemoBin)
	if out, err := exec.Command("schtasks", "/Create", "/TN", controllerWindowsTask, "/TR", command, "/SC", "ONLOGON", "/RL", "LIMITED", "/F").CombinedOutput(); err != nil {
		return "", fmt.Errorf("register Windows controller task: %w: %s", err, out)
	}
	if out, err := exec.Command("schtasks", "/Run", "/TN", controllerWindowsTask).CombinedOutput(); err != nil {
		return "", fmt.Errorf("start Windows controller task: %w: %s", err, out)
	}
	return "Task Scheduler/" + controllerWindowsTask, nil
}

func controllerLaunchAgentPlist(home, mnemoBin string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>%s</string><string>controller</string><string>serve</string></array><key>EnvironmentVariables</key><dict><key>HOME</key><string>%s</string></dict><key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>StandardOutPath</key><string>%s</string><key>StandardErrorPath</key><string>%s</string></dict></plist>
`, controllerLaunchLabel, mnemoBin, home, filepath.Join(home, ".mnemo", "controller.log"), filepath.Join(home, ".mnemo", "controller.err.log"))
}

func controllerSystemdUnit(home, mnemoBin string) string {
	return fmt.Sprintf("[Unit]\nDescription=mnemo global controller\n[Service]\nExecStart=%s controller serve\nEnvironment=HOME=%s\nRestart=on-failure\nRestartSec=1\n[Install]\nWantedBy=default.target\n", mnemoBin, home)
}

func controllerWindowsTaskXML(mnemoBin string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task"><Triggers><LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers><Principals><Principal id="Author"><RunLevel>LeastPrivilege</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><RestartOnFailure><Interval>PT1M</Interval><Count>999</Count></RestartOnFailure><StartWhenAvailable>true</StartWhenAvailable></Settings><Actions Context="Author"><Exec><Command>%s</Command><Arguments>controller serve</Arguments></Exec></Actions></Task>`, mnemoBin)
}
