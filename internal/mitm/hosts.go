package mitm

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	hostsMarkerStart = "# BEGIN AEGIS-PROXY MITM"
	hostsMarkerEnd   = "# END AEGIS-PROXY MITM"
)

type HostsManager struct {
	hostsPath string
}

func NewHostsManager() *HostsManager {
	hostsPath := "/etc/hosts"
	if runtime.GOOS == "windows" {
		hostsPath = "C:\\Windows\\System32\\drivers\\etc\\hosts"
	}
	return &HostsManager{
		hostsPath: hostsPath,
	}
}

func (hm *HostsManager) AddEntry(domain, ip string) error {
	entries, err := hm.GetEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Domain == domain {
			return nil
		}
	}

	entries = append(entries, HostEntry{Domain: domain, IP: ip})
	return hm.writeEntries(entries)
}

func (hm *HostsManager) RemoveEntry(domain string) error {
	entries, err := hm.GetEntries()
	if err != nil {
		return err
	}

	filtered := []HostEntry{}
	for _, entry := range entries {
		if entry.Domain != domain {
			filtered = append(filtered, entry)
		}
	}

	return hm.writeEntries(filtered)
}

func (hm *HostsManager) GetEntries() ([]HostEntry, error) {
	file, err := os.Open(hm.hostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []HostEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open hosts file: %w", err)
	}
	defer file.Close()

	var entries []HostEntry
	scanner := bufio.NewScanner(file)
	inAegisBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == hostsMarkerStart {
			inAegisBlock = true
			continue
		}
		if line == hostsMarkerEnd {
			inAegisBlock = false
			continue
		}

		if inAegisBlock && line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				entries = append(entries, HostEntry{
					IP:     parts[0],
					Domain: parts[1],
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	return entries, nil
}

func (hm *HostsManager) Cleanup() error {
	return hm.writeEntries([]HostEntry{})
}

func (hm *HostsManager) IsSupported() bool {
	_, err := os.Stat(hm.hostsPath)
	return err == nil
}

func (hm *HostsManager) writeEntries(entries []HostEntry) error {
	file, err := os.Open(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("failed to open hosts file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	inAegisBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == hostsMarkerStart {
			inAegisBlock = true
			continue
		}
		if strings.TrimSpace(line) == hostsMarkerEnd {
			inAegisBlock = false
			continue
		}

		if !inAegisBlock {
			lines = append(lines, line)
		}
	}
	file.Close()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	if len(entries) > 0 {
		lines = append(lines, "")
		lines = append(lines, hostsMarkerStart)
		for _, entry := range entries {
			lines = append(lines, fmt.Sprintf("%s %s", entry.IP, entry.Domain))
		}
		lines = append(lines, hostsMarkerEnd)
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(hm.hostsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	return nil
}

type HostEntry struct {
	Domain string
	IP     string
}
