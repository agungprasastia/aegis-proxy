package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/filter"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

func (ps *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Initialize filter engine with default filters
	filterEngine, err := filter.NewFilterEngine(filter.GetDefaultFilters())
	if err != nil {
		writeError(w, r, "Failed to initialize filter engine", http.StatusInternalServerError)
		return
	}
	
	// Apply filter to request messages
	for i := range req.Messages {
		req.Messages[i].Content = filterEngine.ApplyFilter(req.Messages[i].Content)
	}
	
	// Extract client ID for sticky session
	clientID := extractClientID(r)
	
	// Get model info to determine provider
	modelInfo, ok := models.GetModelInfo(req.Model)
	if !ok {
		writeError(w, r, fmt.Sprintf("model not found: %s", req.Model), http.StatusBadRequest)
		return
	}
	
	// Retry logic: try up to 3 times with different accounts
	var prov provider.Provider
	var account *models.Account
	var lastErr error
	
	for attempt := 0; attempt < 3; attempt++ {
		// Get sticky account for this client
		account, err = ps.accountMgr.GetSticky(modelInfo.Provider, clientID)
		if err != nil {
			lastErr = err
			break
		}
		
		// Get provider
		prov, ok = ps.router.GetProvider(modelInfo.Provider)
		if !ok {
			lastErr = fmt.Errorf("provider not found: %s", modelInfo.Provider)
			break
		}
		
		startTime := time.Now()
		
		// Try the request
		var requestErr error
		if req.Stream {
			requestErr = ps.tryStreamingRequest(w, r, prov, account, &req, startTime, filterEngine)
		} else {
			requestErr = ps.tryNonStreamingRequest(w, r, prov, account, &req, startTime, filterEngine)
		}
		
		// If successful, return
		if requestErr == nil {
			return
		}
		
		// If error, mark account and retry
		lastErr = requestErr
		ps.accountMgr.MarkError(account.ID, requestErr.Error())
		ps.logger.Log(&models.RequestLog{
			Model:        req.Model,
			Provider:     prov.Name(),
			AccountID:    &account.ID,
			Status:       "error",
			StatusCode:   http.StatusServiceUnavailable,
			ErrorMessage: requestErr.Error(),
			IPAddress:    getClientIP(r),
			LatencyMs:    int(time.Since(startTime).Milliseconds()),
		})
		
		// Log retry attempt
		if attempt < 2 {
			fmt.Printf("sticky account failed, retrying with different account (attempt %d/3)\n", attempt+1)
		}
	}
	
	// All retries failed
	writeError(w, r, fmt.Sprintf("all retries failed: %v", lastErr), http.StatusServiceUnavailable)
}

// extractClientID extracts a unique client identifier from the request.
// Priority: X-Client-ID header → API key → IP address
func extractClientID(r *http.Request) string {
	// Check X-Client-ID header first
	if clientID := r.Header.Get("X-Client-ID"); clientID != "" {
		return clientID
	}
	
	// Fall back to API key from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	
	// Fall back to IP address
	return getClientIP(r)
}

// tryStreamingRequest attempts a streaming request and returns error if it fails.
func (ps *ProxyServer) tryStreamingRequest(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) error {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          true,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// Call provider's streaming method - it handles SSE writing directly
	err := prov.SendChatCompletionStream(r.Context(), account, chatReq, w)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		// Check for rate limit or auth errors
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
		}
	} else {
		logEntry.Status = "success"
		logEntry.StatusCode = http.StatusOK
	}
	
	ps.logger.Log(logEntry)
	
	// Log to JSONL
	if ps.jsonlLogger != nil {
		ps.jsonlLogger.Log(logger.RequestLogEntry{
			Model:        req.Model,
			Provider:     prov.Name(),
			Status:       logEntry.Status,
			StatusCode:   logEntry.StatusCode,
			Latency:      time.Since(startTime).Nanoseconds(),
			AccountEmail: account.Email,
			Error:        logEntry.ErrorMessage,
		})
	}
	
	return err
}

// tryNonStreamingRequest attempts a non-streaming request and returns error if it fails.
func (ps *ProxyServer) tryNonStreamingRequest(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) error {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          false,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// Call provider's non-streaming method
	chatResp, err := prov.SendChatCompletion(r.Context(), account, chatReq)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		// Check for rate limit or auth errors
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
		}
		
		ps.logger.Log(logEntry)
		return err
	}
	
	// Convert provider.ChatResponse to OpenAIChatResponse
	resp := convertProviderResponseToOpenAI(chatResp)
	
	// Apply reverse filter to response content
	for i := range resp.Choices {
		resp.Choices[i].Message.Content = filterEngine.ReverseFilter(resp.Choices[i].Message.Content)
	}
	
	// Update log with token usage
	logEntry.Status = "success"
	logEntry.StatusCode = http.StatusOK
	logEntry.PromptTokens = resp.Usage.PromptTokens
	logEntry.CompletionTokens = resp.Usage.CompletionTokens
	logEntry.TotalTokens = resp.Usage.TotalTokens
	
	ps.logger.Log(logEntry)
	
	// Log to JSONL
	if ps.jsonlLogger != nil {
		ps.jsonlLogger.Log(logger.RequestLogEntry{
			Model:            req.Model,
			Provider:         prov.Name(),
			Status:           "success",
			StatusCode:       http.StatusOK,
			Latency:          time.Since(startTime).Nanoseconds(),
			AccountEmail:     account.Email,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (ps *ProxyServer) handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          false,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// Call provider's non-streaming method
	chatResp, err := prov.SendChatCompletion(r.Context(), account, chatReq)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		// Check for rate limit or auth errors
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
			writeError(w, r, err.Error(), http.StatusTooManyRequests)
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
			// Mark account as error
			ps.accountMgr.UpdateStatus(account.ID, models.StatusError, err.Error())
			writeError(w, r, "Authentication failed", http.StatusUnauthorized)
		} else {
			writeError(w, r, err.Error(), http.StatusInternalServerError)
		}
		
		ps.logger.Log(logEntry)
		
		// Log to JSONL
		if ps.jsonlLogger != nil {
			ps.jsonlLogger.Log(logger.RequestLogEntry{
				Model:        req.Model,
				Provider:     prov.Name(),
				Status:       "error",
				StatusCode:   logEntry.StatusCode,
				Latency:      time.Since(startTime).Nanoseconds(),
				AccountEmail: account.Email,
				Error:        err.Error(),
			})
		}
		
		return
	}
	
	// Convert provider.ChatResponse to OpenAIChatResponse
	resp := convertProviderResponseToOpenAI(chatResp)
	
	// Apply reverse filter to response content
	for i := range resp.Choices {
		resp.Choices[i].Message.Content = filterEngine.ReverseFilter(resp.Choices[i].Message.Content)
	}
	
	// Update log with token usage
	logEntry.Status = "success"
	logEntry.StatusCode = http.StatusOK
	logEntry.PromptTokens = resp.Usage.PromptTokens
	logEntry.CompletionTokens = resp.Usage.CompletionTokens
	logEntry.TotalTokens = resp.Usage.TotalTokens
	
	ps.logger.Log(logEntry)
	
	// Log to JSONL
	if ps.jsonlLogger != nil {
		ps.jsonlLogger.Log(logger.RequestLogEntry{
			Model:            req.Model,
			Provider:         prov.Name(),
			Status:           "success",
			StatusCode:       http.StatusOK,
			Latency:          time.Since(startTime).Nanoseconds(),
			AccountEmail:     account.Email,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (ps *ProxyServer) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Responses API is just an alias for chat completions
	ps.handleChatCompletions(w, r)
}

func (ps *ProxyServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var anthropicReq AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		writeAnthropicError(w, "Invalid request body", "invalid_request_error", http.StatusBadRequest)
		return
	}
	
	// Initialize filter engine with default filters
	filterEngine, err := filter.NewFilterEngine(filter.GetDefaultFilters())
	if err != nil {
		writeAnthropicError(w, "Failed to initialize filter engine", "internal_error", http.StatusInternalServerError)
		return
	}
	
	// Apply filter to request messages
	for i := range anthropicReq.Messages {
		anthropicReq.Messages[i].Content = filterEngine.ApplyFilter(anthropicReq.Messages[i].Content)
	}
	
	// Apply filter to system message if present
	if anthropicReq.System != "" {
		anthropicReq.System = filterEngine.ApplyFilter(anthropicReq.System)
	}
	
	openaiReq := convertAnthropicToInternal(&anthropicReq)
	
	// Extract client ID for sticky session
	clientID := extractClientID(r)
	
	// Get model info to determine provider
	modelInfo, ok := models.GetModelInfo(openaiReq.Model)
	if !ok {
		writeAnthropicError(w, fmt.Sprintf("model not found: %s", openaiReq.Model), "invalid_request_error", http.StatusBadRequest)
		return
	}
	
	// Retry logic: try up to 3 times with different accounts
	var prov provider.Provider
	var account *models.Account
	var lastErr error
	
	for attempt := 0; attempt < 3; attempt++ {
		// Get sticky account for this client
		account, err = ps.accountMgr.GetSticky(modelInfo.Provider, clientID)
		if err != nil {
			lastErr = err
			break
		}
		
		// Get provider
		prov, ok = ps.router.GetProvider(modelInfo.Provider)
		if !ok {
			lastErr = fmt.Errorf("provider not found: %s", modelInfo.Provider)
			break
		}
		
		startTime := time.Now()
		
		// Try the request
		var requestErr error
		if anthropicReq.Stream {
			ps.handleAnthropicStreaming(w, r, prov, account, openaiReq, startTime, filterEngine)
			return
		} else {
			requestErr = ps.tryAnthropicNonStreaming(w, r, prov, account, openaiReq, startTime, filterEngine)
		}
		
		// If successful, return
		if requestErr == nil {
			return
		}
		
		// If error, mark account and retry
		lastErr = requestErr
		ps.accountMgr.MarkError(account.ID, requestErr.Error())
		ps.logger.Log(&models.RequestLog{
			Model:        openaiReq.Model,
			Provider:     prov.Name(),
			AccountID:    &account.ID,
			Status:       "error",
			StatusCode:   http.StatusServiceUnavailable,
			ErrorMessage: requestErr.Error(),
			IPAddress:    getClientIP(r),
			LatencyMs:    int(time.Since(startTime).Milliseconds()),
		})
		
		// Log retry attempt
		if attempt < 2 {
			fmt.Printf("sticky account failed, retrying with different account (attempt %d/3)\n", attempt+1)
		}
	}
	
	// All retries failed
	writeAnthropicError(w, fmt.Sprintf("all retries failed: %v", lastErr), "service_unavailable", http.StatusServiceUnavailable)
}

// tryAnthropicStreaming attempts an Anthropic streaming request and returns error if it fails.
func (ps *ProxyServer) tryAnthropicStreaming(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) error {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          true,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// We need to intercept the stream to convert OpenAI format to Anthropic format
	// Create a custom response writer that captures the stream
	streamWriter := &anthropicStreamWriter{
		ResponseWriter: w,
		filterEngine:   filterEngine,
		model:          req.Model,
	}
	
	// Set Anthropic SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	// Send message_start event
	fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_%d\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\"}}\n\n", time.Now().Unix(), req.Model)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// Send content_block_start event
	fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// Call provider's streaming method
	err := prov.SendChatCompletionStream(r.Context(), account, chatReq, streamWriter)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Send message_delta and message_stop events
	if err == nil {
		fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		
		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
		}
	} else {
		logEntry.Status = "success"
		logEntry.StatusCode = http.StatusOK
	}
	
	ps.logger.Log(logEntry)
	return err
}

func (ps *ProxyServer) handleAnthropicStreaming(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          true,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// We need to intercept the stream to convert OpenAI format to Anthropic format
	// Create a custom response writer that captures the stream
	streamWriter := &anthropicStreamWriter{
		ResponseWriter: w,
		filterEngine:   filterEngine,
		model:          req.Model,
	}
	
	// Set Anthropic SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	// Send message_start event
	fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_%d\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\"}}\n\n", time.Now().Unix(), req.Model)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// Send content_block_start event
	fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// Call provider's streaming method
	err := prov.SendChatCompletionStream(r.Context(), account, chatReq, streamWriter)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Send message_delta and message_stop events
	if err == nil {
		fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		
		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
			ps.accountMgr.UpdateStatus(account.ID, models.StatusError, err.Error())
		}
	} else {
		logEntry.Status = "success"
		logEntry.StatusCode = http.StatusOK
	}
	
	ps.logger.Log(logEntry)
	
	// Log to JSONL
	if ps.jsonlLogger != nil {
		ps.jsonlLogger.Log(logger.RequestLogEntry{
			Model:        req.Model,
			Provider:     prov.Name(),
			Status:       logEntry.Status,
			StatusCode:   logEntry.StatusCode,
			Latency:      time.Since(startTime).Nanoseconds(),
			AccountEmail: account.Email,
			Error:        logEntry.ErrorMessage,
		})
	}
}

// tryAnthropicNonStreaming attempts an Anthropic non-streaming request and returns error if it fails.
func (ps *ProxyServer) tryAnthropicNonStreaming(w http.ResponseWriter, r *http.Request, prov provider.Provider, account *models.Account, req *OpenAIChatRequest, startTime time.Time, filterEngine *filter.FilterEngine) error {
	// Convert OpenAIChatRequest to provider.ChatRequest
	chatReq := &provider.ChatRequest{
		Model:           req.Model,
		Messages:        convertOpenAIMessagesToProvider(req.Messages),
		Stream:          false,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	
	// Call provider's non-streaming method
	chatResp, err := prov.SendChatCompletion(r.Context(), account, chatReq)
	
	latency := int(time.Since(startTime).Milliseconds())
	
	// Log request
	logEntry := &models.RequestLog{
		Model:      req.Model,
		Provider:   prov.Name(),
		AccountID:  &account.ID,
		LatencyMs:  latency,
		IPAddress:  getClientIP(r),
	}
	
	if err != nil {
		logEntry.Status = "error"
		logEntry.StatusCode = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		
		if strings.Contains(err.Error(), "rate limit") {
			logEntry.StatusCode = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "invalid token") {
			logEntry.StatusCode = http.StatusUnauthorized
		}
		
		ps.logger.Log(logEntry)
		
		// Log to JSONL
		if ps.jsonlLogger != nil {
			ps.jsonlLogger.Log(logger.RequestLogEntry{
				Model:        req.Model,
				Provider:     prov.Name(),
				Status:       "error",
				StatusCode:   logEntry.StatusCode,
				Latency:      time.Since(startTime).Nanoseconds(),
				AccountEmail: account.Email,
				Error:        err.Error(),
			})
		}
		
		return err
	}
	
	// Convert provider.ChatResponse to OpenAIChatResponse first
	openaiResp := convertProviderResponseToOpenAI(chatResp)
	
	// Then convert to Anthropic format
	resp := convertInternalToAnthropic(openaiResp)
	
	// Apply reverse filter to response content
	for i := range resp.Content {
		resp.Content[i].Text = filterEngine.ReverseFilter(resp.Content[i].Text)
	}
	
	// Update log with token usage
	logEntry.Status = "success"
	logEntry.StatusCode = http.StatusOK
	logEntry.PromptTokens = resp.Usage.InputTokens
	logEntry.CompletionTokens = resp.Usage.OutputTokens
	logEntry.TotalTokens = resp.Usage.InputTokens + resp.Usage.OutputTokens
	
	ps.logger.Log(logEntry)
	
	// Log to JSONL
	if ps.jsonlLogger != nil {
		ps.jsonlLogger.Log(logger.RequestLogEntry{
			Model:            req.Model,
			Provider:         prov.Name(),
			Status:           "success",
			StatusCode:       http.StatusOK,
			Latency:          time.Since(startTime).Nanoseconds(),
			AccountEmail:     account.Email,
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (ps *ProxyServer) handleImageGeneration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	writeError(w, r, "Image generation not yet implemented", http.StatusNotImplemented)
}

func (ps *ProxyServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get all models from registry
	allModels := models.ListAllModels()
	
	// Convert to OpenAI model format
	openaiModels := make([]OpenAIModel, 0, len(allModels))
	for _, modelInfo := range allModels {
		openaiModels = append(openaiModels, OpenAIModel{
			ID:       modelInfo.ID,
			Object:   "model",
			OwnedBy:  modelInfo.Provider,
		})
	}
	
	resp := OpenAIModelList{
		Object: "list",
		Data:   openaiModels,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (ps *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	uptime := time.Since(ps.startTime).Seconds()
	
	resp := map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": uptime,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper functions

func convertOpenAIMessagesToProvider(messages []OpenAIMessage) []provider.ChatMessage {
	providerMessages := make([]provider.ChatMessage, len(messages))
	for i, msg := range messages {
		providerMessages[i] = provider.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return providerMessages
}

func convertProviderResponseToOpenAI(resp *provider.ChatResponse) *OpenAIChatResponse {
	choices := make([]OpenAIChoice, len(resp.Choices))
	for i, choice := range resp.Choices {
		choices[i] = OpenAIChoice{
			Index: choice.Index,
			Message: OpenAIMessage{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			},
			FinishReason: choice.FinishReason,
		}
	}
	
	return &OpenAIChatResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: OpenAIUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// anthropicStreamWriter wraps http.ResponseWriter to convert OpenAI SSE to Anthropic SSE
type anthropicStreamWriter struct {
	http.ResponseWriter
	filterEngine *filter.FilterEngine
	model        string
}

func (w *anthropicStreamWriter) Write(p []byte) (int, error) {
	// Parse OpenAI SSE format and convert to Anthropic format
	lines := strings.Split(string(p), "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			if data == "[DONE]" {
				// Skip [DONE] - we'll send message_stop separately
				continue
			}
			
			// Parse OpenAI chunk
			var chunk OpenAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			
			// Convert to Anthropic events
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				
				if choice.Delta != nil && choice.Delta.Content != "" {
					// Apply reverse filter
					content := w.filterEngine.ReverseFilter(choice.Delta.Content)
					
					// Send content_block_delta event
					fmt.Fprintf(w.ResponseWriter, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%s}}\n\n", jsonEscape(content))
					if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
						flusher.Flush()
					}
				}
			}
		}
	}
	
	return len(p), nil
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
