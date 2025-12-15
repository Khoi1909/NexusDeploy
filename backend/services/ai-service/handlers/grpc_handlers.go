package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/services/ai-service/proto"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	cacheKeyPrefix = "ai:analysis:"
	cacheTTL       = 24 * time.Hour
)

// AIServiceServer implements the AIService gRPC server
type AIServiceServer struct {
	proto.UnimplementedAIServiceServer
	cfg         *cfgpkg.Config
	redis       *redis.Client
	buildClient buildpb.BuildServiceClient
	buildConn   *grpc.ClientConn
	httpClient  *http.Client
}

// NewAIServiceServer creates a new AI Service server
func NewAIServiceServer(cfg *cfgpkg.Config, redisClient *redis.Client, buildClient buildpb.BuildServiceClient, buildConn *grpc.ClientConn) *AIServiceServer {
	return &AIServiceServer{
		cfg:         cfg,
		redis:       redisClient,
		buildClient: buildClient,
		buildConn:   buildConn,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// getCorrelationID extracts correlation_id from gRPC metadata for logging
func getCorrelationID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return "unknown"
}

// AnalyzeBuild analyzes build errors using Ollama LLM
func (s *AIServiceServer) AnalyzeBuild(ctx context.Context, req *proto.AnalyzeBuildRequest) (*proto.AnalyzeBuildResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Str("user_plan", req.UserPlan).
		Msg("AnalyzeBuild called")

	if req.BuildId == "" {
		return &proto.AnalyzeBuildResponse{Error: "build_id is required"}, nil
	}

	// Check cache first
	cacheKey := cacheKeyPrefix + req.BuildId
	cachedResult, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil && cachedResult != "" {
		log.Info().
			Str("correlation_id", corrID).
			Str("build_id", req.BuildId).
			Msg("Returning cached analysis result")

		var cached proto.AnalyzeBuildResponse
		if err := json.Unmarshal([]byte(cachedResult), &cached); err == nil {
			cached.Cached = true
			return &cached, nil
		}
	}

	// Get build logs from Build Service
	buildLogsResp, err := s.buildClient.GetBuildLogs(ctx, &buildpb.GetBuildLogsRequest{
		BuildId: req.BuildId,
		Limit:   1000, // Get enough logs for analysis
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("correlation_id", corrID).
			Str("build_id", req.BuildId).
			Msg("Failed to get build logs")
		return &proto.AnalyzeBuildResponse{Error: "failed to get build logs"}, nil
	}

	if buildLogsResp.Error != "" {
		return &proto.AnalyzeBuildResponse{Error: buildLogsResp.Error}, nil
	}

	if len(buildLogsResp.Logs) == 0 {
		return &proto.AnalyzeBuildResponse{Error: "no logs found for this build"}, nil
	}

	// Build prompt from logs
	logLines := make([]string, len(buildLogsResp.Logs))
	for i, logEntry := range buildLogsResp.Logs {
		logLines[i] = logEntry.LogLine
	}
	logsText := strings.Join(logLines, "\n")

	// Build prompt based on user plan
	prompt := s.buildPrompt(req.UserPlan, logsText)

	// Log the full prompt being sent to AI (for debugging)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Str("prompt_length", fmt.Sprintf("%d", len(prompt))).
		Str("prompt_preview", truncateString(prompt, 200)). // First 200 chars
		Msg("Prompt being sent to AI")

	// Log full prompt at DEBUG level (can enable for detailed debugging)
	log.Debug().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Str("full_prompt", prompt).
		Msg("Full prompt sent to AI")

	// Call Ollama API
	analysis, err := s.callOllama(ctx, prompt)
	if err != nil {
		log.Error().
			Err(err).
			Str("correlation_id", corrID).
			Str("build_id", req.BuildId).
			Msg("Failed to call Ollama")
		return &proto.AnalyzeBuildResponse{Error: fmt.Sprintf("failed to analyze: %v", err)}, nil
	}

	// Parse suggestions into array
	suggestionsList := s.parseSuggestions(analysis)

	result := &proto.AnalyzeBuildResponse{
		Analysis:    analysis,
		Suggestions: suggestionsList,
		Cached:      false,
	}

	// Cache the result
	resultJSON, err := json.Marshal(result)
	if err == nil {
		s.redis.Set(ctx, cacheKey, resultJSON, cacheTTL)
	}

	return result, nil
}

// buildPrompt creates a prompt based on user plan
func (s *AIServiceServer) buildPrompt(userPlan, logsText string) string {
	// Context about the platform - critical for AI to understand what users can/cannot do
	platformContext := "You are analyzing build errors from NexusDeploy, a SaaS CI/CD platform. " +
		"Users push code to GitHub, and the platform automatically clones and builds it in managed containers. " +
		"Users CANNOT access containers, cannot run commands, cannot fix permissions, cannot install tools.\n\n" +
		"WHAT USERS CAN DO (ONLY THIS):\n" +
		"- Edit files in their GitHub repository: source code, `package.json`, `Dockerfile`, config files\n" +
		"- Fix code errors: syntax, missing imports, wrong paths, logic errors\n" +
		"- Fix config errors: missing dependencies in `package.json`, wrong build/start commands, Dockerfile mistakes\n" +
		"- Fix repository structure: add missing files, fix directory structure, correct file paths\n\n" +
		"WHAT USERS CANNOT DO (NEVER SUGGEST THESE):\n" +
		"- Run ANY terminal/command line commands (npm install, npm run build, chmod, ls, cd, export, etc.)\n" +
		"- Access or modify the Docker container or CI/CD environment\n" +
		"- Fix permissions, change user privileges, or modify system settings\n" +
		"- Install or configure system tools, npm packages, or dependencies (platform does this automatically)\n" +
		"- Check if tools are installed, verify system setup, or diagnose infrastructure\n" +
		"- Run git commands (code is already in GitHub)\n\n" +
		"CRITICAL: If you suggest commands like 'chmod', 'ls', 'cd', 'npm install', 'export', or any Linux/terminal commands, your response is WRONG.\n\n"

	formatRequirements := "Response format (STRICTLY follow - use markdown):\n" +
		"## Error\n" +
		"[One sentence: what code/config error caused the build to fail]\n\n" +
		"## Fix\n" +
		"1. [Fix in code: specific file and what to change]\n" +
		"2. [Fix in config: specific config file and what to update]\n" +
		"3. [Verify: what to check in code/config to confirm fix]\n\n" +
		"CRITICAL RULES:\n" +
		"- Error: EXACTLY 1 sentence only\n" +
		"- Fix: EXACTLY 3 steps, each is ONE sentence\n" +
		"- Total: UNDER 60 words\n" +
		"- Use inline code with backticks for file names: `package.json`, `Dockerfile`\n" +
		"- NO code blocks, NO multiline commands\n" +
		"- NO URLs, NO links\n" +
		"- Be specific: mention exact file names, function names, config keys\n" +
		"- NEVER suggest ANY commands: NO 'npm install', NO 'npm run build', NO 'chmod', NO 'ls', NO 'cd', NO 'export', NO Linux commands\n" +
		"- NEVER suggest checking/verifying system: NO 'check permissions', NO 'verify npm installed', NO 'check directory structure'\n" +
		"- NEVER suggest container/system access: NO 'inside docker', NO 'in container', NO 'check environment'\n" +
		"- ONLY suggest editing files in GitHub repository: `package.json`, `Dockerfile`, source code, config files\n" +
		"- EVERY fix step MUST be about editing a file in the repo, NOT running commands\n\n"

	goodExample := "Good example:\n" +
		"## Error\n" +
		"npm build fails because package.json is missing the \"build\" script in scripts section.\n\n" +
		"## Fix\n" +
		"1. Add \"build\" script to `package.json` scripts section with your build command.\n" +
		"2. Ensure all dependencies in `package.json` dependencies field are valid npm packages.\n" +
		"3. Commit and push updated `package.json` to GitHub to trigger a new build.\n\n"

	badExample := "Bad example (DO NOT DO THIS):\n" +
		"## Error\n" +
		"The build command is not running successfully due to permissions.\n\n" +
		"## Fix\n" +
		"1. Check permissions using 'chmod -R 755 /tmp/nexus-builds/*'\n" +
		"2. Use 'ls' command to verify directory structure\n" +
		"3. Run 'export NODE_OPTIONS=...' to configure environment\n\n" +
		"This is COMPLETELY WRONG because:\n" +
		"- Users CANNOT run ANY commands (chmod, ls, export, npm install, npm run build, etc.)\n" +
		"- Users CANNOT access containers or system permissions\n" +
		"- Users CANNOT modify environment variables or system settings\n" +
		"- Platform handles all of this automatically - users only edit files in their repo\n\n" +
		"Correct approach for build command errors:\n" +
		"## Error\n" +
		"Build command fails because `package.json` scripts section has wrong command or missing script.\n\n" +
		"## Fix\n" +
		"1. Check `package.json` scripts section - ensure build script exists and command is correct.\n" +
		"2. Verify all dependencies in `package.json` dependencies field are valid and listed correctly.\n" +
		"3. Check `Dockerfile` COPY commands match your project structure and file paths.\n\n"

	basePrompt := platformContext + formatRequirements + goodExample + badExample

	if userPlan == "premium" {
		// Premium plan: More detailed analysis
		return fmt.Sprintf(`%sFor premium users: You may provide more detailed explanations and alternative solutions if applicable.

Build logs:
%s`, basePrompt, logsText)
	}

	// Standard plan: Concise and direct
	return fmt.Sprintf(`%sBuild logs:
%s`, basePrompt, logsText)
}

// callOllama calls the Ollama API to get analysis
func (s *AIServiceServer) callOllama(ctx context.Context, prompt string) (string, error) {
	url := s.cfg.LLMAPIURL
	if url == "" {
		url = "http://ollama:11434/api/generate"
	}

	model := s.cfg.LLMModel
	if model == "" {
		model = "deepseek-coder"
	}

	requestBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var ollamaResp struct {
		Response string `json:"response"`
		Error    string `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("ollama error: %s", ollamaResp.Error)
	}

	return ollamaResp.Response, nil
}

// parseSuggestions extracts suggestions from analysis text
func (s *AIServiceServer) parseSuggestions(analysis string) []string {
	lines := strings.Split(analysis, "\n")
	suggestions := []string{}
	inFixSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for "Fix:" section (case insensitive, with or without bold)
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "fix:") || strings.Contains(lowerLine, "**fix:**") {
			inFixSection = true
			continue
		}

		// Stop at next major section (if we hit another bold section that's not fix-related)
		if inFixSection && strings.HasPrefix(line, "**") && !strings.Contains(lowerLine, "fix") {
			break
		}

		// Extract numbered list items (1., 2., 3., etc.) from Fix section
		if inFixSection {
			// Match patterns like "1.", "2.", "1)", "2)", "- ", "* "
			if len(line) >= 2 {
				// Check for numbered list (1. 2. 3. etc)
				if line[0] >= '1' && line[0] <= '5' && (line[1] == '.' || line[1] == ')') {
					suggestion := strings.TrimSpace(line[2:])
					// Remove markdown links and URLs
					suggestion = s.cleanSuggestion(suggestion)
					if suggestion != "" && len(suggestion) > 5 {
						suggestions = append(suggestions, suggestion)
					}
					continue
				}
			}
			// Check for bullet points
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				suggestion := strings.TrimSpace(line[2:])
				suggestion = s.cleanSuggestion(suggestion)
				if suggestion != "" && len(suggestion) > 5 {
					suggestions = append(suggestions, suggestion)
				}
			}
		}
	}

	// If no structured suggestions found in Fix section, try extracting any numbered list from the whole text
	if len(suggestions) == 0 {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) >= 2 && line[0] >= '1' && line[0] <= '9' && (line[1] == '.' || line[1] == ')') {
				suggestion := strings.TrimSpace(line[2:])
				suggestion = s.cleanSuggestion(suggestion)
				if suggestion != "" && len(suggestion) > 5 {
					suggestions = append(suggestions, suggestion)
				}
				if len(suggestions) >= 5 {
					break
				}
			}
		}
	}

	// Limit to 5 suggestions max
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// truncateString truncates a string to maxLength and adds "..." if truncated
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// cleanSuggestion removes markdown links, URLs, and other unwanted text
func (s *AIServiceServer) cleanSuggestion(suggestion string) string {
	// Remove markdown links [text](url)
	re := strings.NewReplacer(
		"[step ", "",
		"](https://", "",
		"](http://", "",
		"](github.com/", "",
		"step 1", "",
		"step 2", "",
		"step 3", "",
		"step 4", "",
		"step 5", "",
	)
	suggestion = re.Replace(suggestion)

	// Remove URLs
	if idx := strings.Index(suggestion, "http://"); idx != -1 {
		suggestion = suggestion[:idx]
	}
	if idx := strings.Index(suggestion, "https://"); idx != -1 {
		suggestion = suggestion[:idx]
	}
	if idx := strings.Index(suggestion, "github.com/"); idx != -1 {
		suggestion = suggestion[:idx]
	}

	// Remove markdown formatting
	suggestion = strings.ReplaceAll(suggestion, "**", "")
	suggestion = strings.ReplaceAll(suggestion, "*", "")
	suggestion = strings.ReplaceAll(suggestion, "_", "")
	suggestion = strings.ReplaceAll(suggestion, "`", "")

	// Clean up extra whitespace
	suggestion = strings.TrimSpace(suggestion)
	words := strings.Fields(suggestion)
	suggestion = strings.Join(words, " ")

	return suggestion
}
