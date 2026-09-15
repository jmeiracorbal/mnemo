package main

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const controllerLaunchLabel = "com.jmeiracorbal.mnemo.controller"
const controllerSystemdName = "mnemo-controller.service"
const controllerWindowsTask = "MnemoController"

//go:embed assets/controller-task.xml
var controllerWindowsTaskTemplate string

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
	path := filepath.Join(home, ".mnemo", "controller-task.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(controllerWindowsTaskXML(mnemoBin)), 0644); err != nil {
		return "", err
	}
	if out, err := exec.Command("schtasks", "/Create", "/TN", controllerWindowsTask, "/XML", path, "/F").CombinedOutput(); err != nil {
		return "", fmt.Errorf("register Windows controller task: %w: %s", err, out)
	}
	if out, err := exec.Command("schtasks", "/Run", "/TN", controllerWindowsTask).CombinedOutput(); err != nil {
		return "", fmt.Errorf("start Windows controller task: %w: %s", err, out)
	}
	return path, nil
}

// uninstallControllerService removes the installation-owned controller. It is
// intentionally coupled to setup uninstall: removing an agent integration must
// not leave a background process owning the user's database.
func uninstallControllerService(home string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", controllerLaunchLabel+".plist")
		domain := "gui/" + strconv.Itoa(os.Getuid())
		_ = exec.Command("launchctl", "bootout", domain, path).Run()
		if err := removeIfExists(path); err != nil {
			return "", err
		}
		return path, nil
	case "linux":
		path := filepath.Join(home, ".config", "systemd", "user", controllerSystemdName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "", nil
		} else if err != nil {
			return "", err
		}
		if out, err := exec.Command("systemctl", "--user", "disable", "--now", controllerSystemdName).CombinedOutput(); err != nil {
			return "", fmt.Errorf("stop systemd controller service: %w: %s", err, out)
		}
		if err := removeIfExists(path); err != nil {
			return "", err
		}
		if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
			return "", fmt.Errorf("reload systemd units: %w: %s", err, out)
		}
		return path, nil
	case "windows":
		path := filepath.Join(home, ".mnemo", "controller-task.xml")
		if out, err := exec.Command("schtasks", "/Delete", "/TN", controllerWindowsTask, "/F").CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "cannot find") {
			return "", fmt.Errorf("remove Windows controller task: %w: %s", err, out)
		}
		if err := removeIfExists(path); err != nil {
			return "", err
		}
		return path, nil
	default:
		return "", fmt.Errorf("persistent controller service is not supported on %s", runtime.GOOS)
	}
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func controllerLaunchAgentPlist(home, mnemoBin string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>%s</string><string>controller</string><string>serve</string></array><key>EnvironmentVariables</key><dict><key>HOME</key><string>%s</string></dict><key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>StandardOutPath</key><string>%s</string><key>StandardErrorPath</key><string>%s</string></dict></plist>
`, xmlEscape(controllerLaunchLabel), xmlEscape(mnemoBin), xmlEscape(home), xmlEscape(filepath.Join(home, ".mnemo", "controller.log")), xmlEscape(filepath.Join(home, ".mnemo", "controller.err.log")))
}

func controllerSystemdUnit(home, mnemoBin string) string {
	return fmt.Sprintf("[Unit]\nDescription=mnemo global controller\n[Service]\nExecStart=%s controller serve\nEnvironment=%s\nRestart=on-failure\nRestartSec=1\n[Install]\nWantedBy=default.target\n", strconv.Quote(mnemoBin), strconv.Quote("HOME="+home))
}

func controllerWindowsTaskXML(mnemoBin string) string {
	return strings.ReplaceAll(controllerWindowsTaskTemplate, "{{MNEMO_BIN}}", xmlEscape(mnemoBin))
}

func xmlEscape(value string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}
