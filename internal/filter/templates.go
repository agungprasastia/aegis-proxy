package filter

import (
	"fmt"
	"log"
	"regexp"
)

// FilterTemplate represents a named collection of filter rules
type FilterTemplate struct {
	Name  string
	Rules []FilterRule
}

// GetBuiltinTemplates returns all built-in filter templates
func GetBuiltinTemplates() map[string]FilterTemplate {
	return map[string]FilterTemplate{
		"basic":      getBasicTemplate(),
		"aggressive": getAggressiveTemplate(),
		"minimal":    getMinimalTemplate(),
	}
}

// getBasicTemplate returns the basic template with 9 rules (current default filters)
func getBasicTemplate() FilterTemplate {
	return FilterTemplate{
		Name: "basic",
		Rules: []FilterRule{
			{
				ID:            "basic_anthropic",
				Pattern:       "Anthropic",
				Replacement:   "Anxthxropic",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_claude",
				Pattern:       "Claude",
				Replacement:   "Clxude",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_openai",
				Pattern:       "OpenAI",
				Replacement:   "0penAI",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_gpt",
				Pattern:       "GPT",
				Replacement:   "GxPT",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_cc_version",
				Pattern:       "cc_version",
				Replacement:   "app_version",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_claude_prefix",
				Pattern:       "claude-",
				Replacement:   "model-",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_gpt_prefix",
				Pattern:       "gpt-",
				Replacement:   "llm-",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_anthropic_api",
				Pattern:       "api.anxthxropic.com",
				Replacement:   "api.example.com",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "basic_openai_api",
				Pattern:       "api.openai.com",
				Replacement:   "api.example.com",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
		},
	}
}

// getAggressiveTemplate returns the aggressive template with 32 rules from enowxai forensics
func getAggressiveTemplate() FilterTemplate {
	return FilterTemplate{
		Name: "aggressive",
		Rules: []FilterRule{
			// Basic rules (from basic template)
			{
				ID:            "agg_anthropic",
				Pattern:       "Anthropic",
				Replacement:   "Anxthxropic",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_claude",
				Pattern:       "Claude",
				Replacement:   "CL4ude",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_openai",
				Pattern:       "OpenAI",
				Replacement:   "0penAI",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_gpt",
				Pattern:       "GPT",
				Replacement:   "GxPT",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			// Advanced obfuscation rules from enowxai forensics
			{
				ID:            "agg_claude_code",
				Pattern:       "Claude Code",
				Replacement:   "Code Assistant",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_jailbreak",
				Pattern:       "Jailbreak",
				Replacement:   "J41lbreak",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_godmode",
				Pattern:       "godmode",
				Replacement:   "g0dmode",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_prompt_injection",
				Pattern:       "prompt injection",
				Replacement:   "prompt inj3ction",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_red_teaming",
				Pattern:       "red-teaming",
				Replacement:   "red-team1ng",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_cache_control",
				Pattern:       "cache_control",
				Replacement:   "",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_cc_entrypoint",
				Pattern:       "cc_entrypoint",
				Replacement:   "app_entrypoint",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_sensitive_content",
				Pattern:       "sensitive_content",
				Replacement:   "flagged_content",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_system_prompt",
				Pattern:       "system_prompt",
				Replacement:   "sys_msg",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_assistant",
				Pattern:       "assistant",
				Replacement:   "4ssistant",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_ai_model",
				Pattern:       "AI model",
				Replacement:   "ML system",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_language_model",
				Pattern:       "language model",
				Replacement:   "text processor",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_neural_network",
				Pattern:       "neural network",
				Replacement:   "n3ural n3twork",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_machine_learning",
				Pattern:       "machine learning",
				Replacement:   "ML processing",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_deep_learning",
				Pattern:       "deep learning",
				Replacement:   "d33p learning",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_transformer",
				Pattern:       "transformer",
				Replacement:   "tr4nsformer",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_token_limit",
				Pattern:       "token limit",
				Replacement:   "context window",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_context_window",
				Pattern:       "context window",
				Replacement:   "input buffer",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_fine_tuning",
				Pattern:       "fine-tuning",
				Replacement:   "f1ne-tuning",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_training_data",
				Pattern:       "training data",
				Replacement:   "tr4ining data",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_api_key",
				Pattern:       "API key",
				Replacement:   "auth token",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_rate_limit",
				Pattern:       "rate limit",
				Replacement:   "r4te limit",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_model_version",
				Pattern:       "model version",
				Replacement:   "system version",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_inference",
				Pattern:       "inference",
				Replacement:   "1nference",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_embedding",
				Pattern:       "embedding",
				Replacement:   "3mbedding",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_vector_database",
				Pattern:       "vector database",
				Replacement:   "v3ctor database",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_prompt_engineering",
				Pattern:       "prompt engineering",
				Replacement:   "input optimization",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "agg_temperature",
				Pattern:       "temperature",
				Replacement:   "t3mperature",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
		},
	}
}

// getMinimalTemplate returns the minimal template with 3 rules (subset of basic)
func getMinimalTemplate() FilterTemplate {
	return FilterTemplate{
		Name: "minimal",
		Rules: []FilterRule{
			{
				ID:            "min_claude",
				Pattern:       "Claude",
				Replacement:   "CL4ude",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "min_godmode",
				Pattern:       "godmode",
				Replacement:   "g0dmode",
				IsRegex:       false,
				CaseSensitive: false,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
			{
				ID:            "min_cache_control",
				Pattern:       "cache_control",
				Replacement:   "",
				IsRegex:       false,
				CaseSensitive: true,
				IsActive:      true,
				Mode:          FilterModeBoth,
			},
		},
	}
}

// LoadTemplate replaces the current filters with a template by name
func (e *FilterEngine) LoadTemplate(name string) error {
	templates := GetBuiltinTemplates()
	template, exists := templates[name]
	if !exists {
		return fmt.Errorf("template '%s' not found", name)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing filters
	e.filters = make(map[string]*FilterRule)

	// Add template rules
	for _, rule := range template.Rules {
		// Compile regex patterns if needed
		if rule.IsRegex {
			flags := ""
			if !rule.CaseSensitive {
				flags = "(?i)"
			}
			compiled, err := regexp.Compile(flags + rule.Pattern)
			if err != nil {
				return fmt.Errorf("failed to compile regex pattern for rule %s: %w", rule.ID, err)
			}
			rule.compiled = compiled
		}

		// Store filter
		ruleCopy := rule
		e.filters[rule.ID] = &ruleCopy
	}

	log.Printf("Loaded template '%s' with %d rules", name, len(template.Rules))
	return nil
}
