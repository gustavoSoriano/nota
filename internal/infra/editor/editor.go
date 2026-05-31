package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Open(editor, filePath string) error {
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CreateTempFile(content string) (string, error) {
	dir := os.TempDir()
	f, err := os.CreateTemp(dir, "nota-*.md")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func ReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Cleanup(path string) {
	os.Remove(path)
}

func Detect() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	candidates := []string{"micro", "vim", "nano", "vi"}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return "vi"
}

func IsInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func InstallMicro() error {
	switch runtime.GOOS {
	case "darwin":
		return run("brew", "install", "micro")
	case "linux":
		return installMicroLinux()
	default:
		return fmt.Errorf("unsupported OS for auto-install: %s", runtime.GOOS)
	}
}

func installMicroLinux() error {
	home, _ := os.UserHomeDir()
	dest := filepath.Join(home, ".local", "bin", "micro")
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	return run("curl", "https://getmic.ro", "|", "bash")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
