package mitm

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

type TrustStoreManager struct {
	certPath string
}

func NewTrustStoreManager(certPath string) *TrustStoreManager {
	return &TrustStoreManager{
		certPath: certPath,
	}
}

func (tsm *TrustStoreManager) InstallCert(certPath string) error {
	switch runtime.GOOS {
	case "windows":
		return tsm.installWindows(certPath)
	case "darwin":
		return tsm.installMacOS(certPath)
	case "linux":
		return tsm.installLinux(certPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (tsm *TrustStoreManager) UninstallCert() error {
	switch runtime.GOOS {
	case "windows":
		return tsm.uninstallWindows()
	case "darwin":
		return tsm.uninstallMacOS()
	case "linux":
		return tsm.uninstallLinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (tsm *TrustStoreManager) IsCertInstalled() bool {
	switch runtime.GOOS {
	case "windows":
		return tsm.isInstalledWindows()
	case "darwin":
		return tsm.isInstalledMacOS()
	case "linux":
		return tsm.isInstalledLinux()
	default:
		return false
	}
}

func (tsm *TrustStoreManager) GetTrustStorePath() string {
	switch runtime.GOOS {
	case "windows":
		return "Cert:\\CurrentUser\\Root"
	case "darwin":
		return "/Library/Keychains/System.keychain"
	case "linux":
		return "/usr/local/share/ca-certificates"
	default:
		return ""
	}
}

func (tsm *TrustStoreManager) installWindows(certPath string) error {
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (tsm *TrustStoreManager) uninstallWindows() error {
	cmd := exec.Command("certutil", "-delstore", "-user", "Root", "Aegis Proxy Root CA")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (tsm *TrustStoreManager) isInstalledWindows() bool {
	cmd := exec.Command("certutil", "-store", "-user", "Root")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return containsString(string(output), "Aegis Proxy Root CA")
}

func (tsm *TrustStoreManager) installMacOS(certPath string) error {
	cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", certPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (tsm *TrustStoreManager) uninstallMacOS() error {
	cmd := exec.Command("sudo", "security", "delete-certificate", "-c", "Aegis Proxy Root CA", "/Library/Keychains/System.keychain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (tsm *TrustStoreManager) isInstalledMacOS() bool {
	cmd := exec.Command("security", "find-certificate", "-c", "Aegis Proxy Root CA", "/Library/Keychains/System.keychain")
	err := cmd.Run()
	return err == nil
}

func (tsm *TrustStoreManager) installLinux(certPath string) error {
	destPath := "/usr/local/share/ca-certificates/aegis-proxy-ca.crt"
	
	data, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to copy certificate: %w", err)
	}

	cmd := exec.Command("update-ca-certificates")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update CA certificates: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (tsm *TrustStoreManager) uninstallLinux() error {
	destPath := "/usr/local/share/ca-certificates/aegis-proxy-ca.crt"
	
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove certificate: %w", err)
	}

	cmd := exec.Command("update-ca-certificates", "--fresh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update CA certificates: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (tsm *TrustStoreManager) isInstalledLinux() bool {
	destPath := "/usr/local/share/ca-certificates/aegis-proxy-ca.crt"
	_, err := os.Stat(destPath)
	return err == nil
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsString(s[1:], substr)))
}
