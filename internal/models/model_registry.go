package models

type ModelInfo struct {
	ID       string
	Name     string
	Provider string
	Tier     string
}

var Registry = map[string]ModelInfo{
	"auto": {
		ID:       "auto",
		Name:     "Auto (Best Available)",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"claude-sonnet-4.5": {
		ID:       "claude-sonnet-4.5",
		Name:     "Claude Sonnet 4.5",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"claude-sonnet-4": {
		ID:       "claude-sonnet-4",
		Name:     "Claude Sonnet 4",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"claude-haiku-4.5": {
		ID:       "claude-haiku-4.5",
		Name:     "Claude Haiku 4.5",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"deepseek-3.2": {
		ID:       "deepseek-3.2",
		Name:     "DeepSeek 3.2",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"minimax-m2.5": {
		ID:       "minimax-m2.5",
		Name:     "MiniMax M2.5",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"glm-5": {
		ID:       "glm-5",
		Name:     "GLM-5",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"qwen3-coder-next": {
		ID:       "qwen3-coder-next",
		Name:     "Qwen3 Coder Next",
		Provider: ProviderKiro,
		Tier:     "standard",
	},
	"claude-opus-4.6": {
		ID:       "claude-opus-4.6",
		Name:     "Claude Opus 4.6",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"gemini-2.5-pro": {
		ID:       "gemini-2.5-pro",
		Name:     "Gemini 2.5 Pro",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"gemini-2.5-flash": {
		ID:       "gemini-2.5-flash",
		Name:     "Gemini 2.5 Flash",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"gpt-5.4": {
		ID:       "gpt-5.4",
		Name:     "GPT-5.4",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"gpt-5.2": {
		ID:       "gpt-5.2",
		Name:     "GPT-5.2",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"deepseek-v3-2": {
		ID:       "deepseek-v3-2",
		Name:     "DeepSeek V3-2",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"kimi-k2.5": {
		ID:       "kimi-k2.5",
		Name:     "Kimi K2.5",
		Provider: ProviderCodeBuddy,
		Tier:     "max",
	},
	"windsurf-claude-sonnet-4": {
		ID:       "windsurf-claude-sonnet-4",
		Name:     "Windsurf Claude Sonnet 4",
		Provider: ProviderWindsurf,
		Tier:     "standard",
	},
	"windsurf-gpt-5": {
		ID:       "windsurf-gpt-5",
		Name:     "Windsurf GPT-5",
		Provider: ProviderWindsurf,
		Tier:     "standard",
	},
	"canva-image": {
		ID:       "canva-image",
		Name:     "Canva Image Generation",
		Provider: ProviderCanva,
		Tier:     "standard",
	},
	"yepapi-default": {
		ID:       "yepapi-default",
		Name:     "YepAPI Default",
		Provider: ProviderYepAPI,
		Tier:     "standard",
	},
	"codex-default": {
		ID:       "codex-default",
		Name:     "Codex Default",
		Provider: ProviderCodex,
		Tier:     "standard",
	},
	"wavespeed-claude-sonnet-4.5": {
		ID:       "wavespeed-claude-sonnet-4.5",
		Name:     "Wavespeed Claude Sonnet 4.5",
		Provider: ProviderWavespeed,
		Tier:     "standard",
	},
	"wavespeed-claude-sonnet-4": {
		ID:       "wavespeed-claude-sonnet-4",
		Name:     "Wavespeed Claude Sonnet 4",
		Provider: ProviderWavespeed,
		Tier:     "standard",
	},
	"wavespeed-gpt-5": {
		ID:       "wavespeed-gpt-5",
		Name:     "Wavespeed GPT-5",
		Provider: ProviderWavespeed,
		Tier:     "standard",
	},
	"deepseek-v3": {
		ID:       "deepseek-v3",
		Name:     "DeepSeek V3",
		Provider: ProviderDeepSeek,
		Tier:     "apikey",
	},
	"deepseek-r1": {
		ID:       "deepseek-r1",
		Name:     "DeepSeek R1",
		Provider: ProviderDeepSeek,
		Tier:     "apikey",
	},
	"groq-default": {
		ID:       "groq-default",
		Name:     "Groq Fast Inference",
		Provider: ProviderGroq,
		Tier:     "apikey",
	},
	"glm-5-apikey": {
		ID:       "glm-5-apikey",
		Name:     "GLM-5",
		Provider: ProviderGLM,
		Tier:     "apikey",
	},
	"glm-4.7": {
		ID:       "glm-4.7",
		Name:     "GLM-4.7",
		Provider: ProviderGLM,
		Tier:     "apikey",
	},
	"minimax-m2.7": {
		ID:       "minimax-m2.7",
		Name:     "MiniMax M2.7",
		Provider: ProviderMiniMax,
		Tier:     "apikey",
	},
	"minimax-m2.5-apikey": {
		ID:       "minimax-m2.5-apikey",
		Name:     "MiniMax M2.5",
		Provider: ProviderMiniMax,
		Tier:     "apikey",
	},
	"mistral-large": {
		ID:       "mistral-large",
		Name:     "Mistral Large",
		Provider: ProviderMistral,
		Tier:     "apikey",
	},
	"codestral": {
		ID:       "codestral",
		Name:     "Codestral",
		Provider: ProviderMistral,
		Tier:     "apikey",
	},
	"grok-default": {
		ID:       "grok-default",
		Name:     "Grok",
		Provider: ProviderXAI,
		Tier:     "apikey",
	},
}

func GetModelInfo(id string) (ModelInfo, bool) {
	info, ok := Registry[id]
	return info, ok
}

func GetModelsForProvider(provider string) []ModelInfo {
	var models []ModelInfo
	for _, info := range Registry {
		if info.Provider == provider {
			models = append(models, info)
		}
	}
	return models
}

func ListAllModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(Registry))
	for _, info := range Registry {
		models = append(models, info)
	}
	return models
}
