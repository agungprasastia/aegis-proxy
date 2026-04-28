package filter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// FilterMode defines how a filter is applied
type FilterMode string

const (
	FilterModeBoth         FilterMode = "both"
	FilterModeRequestOnly  FilterMode = "request_only"
	FilterModeResponseOnly FilterMode = "response_only"
	FilterModeDisabled     FilterMode = "disabled"
)

// FilterRule represents a single text replacement rule
type FilterRule struct {
	ID            string     // Unique identifier
	Pattern       string     // Original pattern (can be regex or literal)
	Replacement   string     // Replacement text
	IsRegex       bool       // Whether pattern is regex
	CaseSensitive bool       // Case sensitivity
	IsActive      bool       // Whether filter is active
	Mode          FilterMode // Filter mode (both/request_only/response_only/disabled)
	compiled      *regexp.Regexp
}

// FilterEngine manages and applies content filters
type FilterEngine struct {
	mu      sync.RWMutex
	filters map[string]*FilterRule // Map ID -> FilterRule
}

// NewFilterEngine creates a new filter engine with the given rules
func NewFilterEngine(filters []FilterRule) (*FilterEngine, error) {
	engine := &FilterEngine{
		filters: make(map[string]*FilterRule),
	}

	// Add all filters
	for _, filter := range filters {
		// Generate ID if not provided
		if filter.ID == "" {
			filter.ID = generateFilterID(filter.Pattern, filter.Replacement)
		}
		// Set defaults
		if filter.Mode == "" {
			filter.Mode = FilterModeBoth
		}
		
		// Compile regex patterns
		if filter.IsRegex {
			flags := ""
			if !filter.CaseSensitive {
				flags = "(?i)"
			}
			compiled, err := regexp.Compile(flags + filter.Pattern)
			if err != nil {
				return nil, err
			}
			filter.compiled = compiled
		}
		
		// Store filter
		filterCopy := filter
		engine.filters[filter.ID] = &filterCopy
	}

	return engine, nil
}

// generateFilterID generates a unique ID for a filter based on pattern and replacement
func generateFilterID(pattern, replacement string) string {
	hash := sha256.Sum256([]byte(pattern + ":" + replacement))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
}

// ApplyFilter applies forward text replacements to the input text
func (e *FilterEngine) ApplyFilter(text string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := text
	for _, filter := range e.filters {
		// Skip if inactive or mode is response_only/disabled
		if !filter.IsActive || filter.Mode == FilterModeResponseOnly || filter.Mode == FilterModeDisabled {
			continue
		}
		
		if filter.IsRegex && filter.compiled != nil {
			result = filter.compiled.ReplaceAllString(result, filter.Replacement)
		} else {
			// Literal string replacement
			if filter.CaseSensitive {
				result = strings.ReplaceAll(result, filter.Pattern, filter.Replacement)
			} else {
				// Case-insensitive literal replacement
				result = caseInsensitiveReplace(result, filter.Pattern, filter.Replacement)
			}
		}
	}

	return result
}

// ReverseFilter applies reverse text replacements to the input text
func (e *FilterEngine) ReverseFilter(text string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := text
	// Apply filters in reverse order, swapping pattern and replacement
	// Note: We need to iterate in reverse order for proper reversal
	filterList := make([]*FilterRule, 0, len(e.filters))
	for _, filter := range e.filters {
		filterList = append(filterList, filter)
	}
	
	for i := len(filterList) - 1; i >= 0; i-- {
		filter := filterList[i]
		
		// Skip if inactive or mode is request_only/disabled
		if !filter.IsActive || filter.Mode == FilterModeRequestOnly || filter.Mode == FilterModeDisabled {
			continue
		}
		
		if filter.IsRegex {
			// For regex, we can't simply reverse, so we skip
			// Regex reverse replacement would require storing original matches
			continue
		}

		// Literal string replacement (swap pattern and replacement)
		if filter.CaseSensitive {
			result = strings.ReplaceAll(result, filter.Replacement, filter.Pattern)
		} else {
			result = caseInsensitiveReplace(result, filter.Replacement, filter.Pattern)
		}
	}

	return result
}

// AddFilter adds a new filter rule to the engine
func (e *FilterEngine) AddFilter(rule FilterRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate ID if not provided
	if rule.ID == "" {
		rule.ID = generateFilterID(rule.Pattern, rule.Replacement)
	}

	// Set defaults
	if rule.Mode == "" {
		rule.Mode = FilterModeBoth
	}

	// Compile regex patterns
	if rule.IsRegex {
		flags := ""
		if !rule.CaseSensitive {
			flags = "(?i)"
		}
		compiled, err := regexp.Compile(flags + rule.Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile regex pattern: %w", err)
		}
		rule.compiled = compiled
	}

	// Store filter
	e.filters[rule.ID] = &rule
	return nil
}

// RemoveFilter removes a filter rule by ID
func (e *FilterEngine) RemoveFilter(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.filters[id]; !exists {
		return fmt.Errorf("filter with ID %s not found", id)
	}

	delete(e.filters, id)
	return nil
}

// ToggleFilter toggles the IsActive flag for a filter by ID
func (e *FilterEngine) ToggleFilter(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	filter, exists := e.filters[id]
	if !exists {
		return fmt.Errorf("filter with ID %s not found", id)
	}

	filter.IsActive = !filter.IsActive
	return nil
}

// SetMode sets the FilterMode for a filter by ID
func (e *FilterEngine) SetMode(id string, mode FilterMode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	filter, exists := e.filters[id]
	if !exists {
		return fmt.Errorf("filter with ID %s not found", id)
	}

	// Validate mode
	switch mode {
	case FilterModeBoth, FilterModeRequestOnly, FilterModeResponseOnly, FilterModeDisabled:
		filter.Mode = mode
	default:
		return fmt.Errorf("invalid filter mode: %s", mode)
	}

	return nil
}

// GetFilter retrieves a filter by ID
func (e *FilterEngine) GetFilter(id string) (*FilterRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filter, exists := e.filters[id]
	if !exists {
		return nil, fmt.Errorf("filter with ID %s not found", id)
	}

	// Return a copy to prevent external modification
	filterCopy := *filter
	return &filterCopy, nil
}

// ListFilters returns all filter rules
func (e *FilterEngine) ListFilters() []FilterRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filters := make([]FilterRule, 0, len(e.filters))
	for _, filter := range e.filters {
		filterCopy := *filter
		filters = append(filters, filterCopy)
	}

	return filters
}

// caseInsensitiveReplace performs case-insensitive string replacement
func caseInsensitiveReplace(text, old, new string) string {
	if old == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerOld := strings.ToLower(old)
	
	var result strings.Builder
	result.Grow(len(text))
	
	lastIndex := 0
	for {
		index := strings.Index(lowerText[lastIndex:], lowerOld)
		if index == -1 {
			result.WriteString(text[lastIndex:])
			break
		}
		
		actualIndex := lastIndex + index
		result.WriteString(text[lastIndex:actualIndex])
		result.WriteString(new)
		lastIndex = actualIndex + len(old)
	}
	
	return result.String()
}

// isValidMode checks if a filter mode is valid
func isValidMode(mode FilterMode) bool {
	switch mode {
	case FilterModeBoth, FilterModeRequestOnly, FilterModeResponseOnly, FilterModeDisabled:
		return true
	default:
		return false
	}
}
