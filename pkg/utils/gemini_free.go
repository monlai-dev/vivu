package utils

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"hash/fnv"
	"log"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
	"vivu/internal/models/request_models"

	"github.com/google/generative-ai-go/genai"
	"github.com/pgvector/pgvector-go"
	"google.golang.org/api/option"
)

// GeminiEmbeddingClient implements EmbeddingClientInterface using Google's Gemini models
type GeminiEmbeddingClient struct {
	client *genai.Client
	model  string
}

// NewGeminiEmbeddingClient creates a new Gemini client
func NewGeminiEmbeddingClient(apiKey, model string) (EmbeddingClientInterface, error) {
	if model == "" {
		model = "gemini-1.5-flash" // Free tier model
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiEmbeddingClient{
		client: client,
		model:  model,
	}, nil
}

func (c *GeminiEmbeddingClient) GeneratePlanOnlyJSON(
	ctx context.Context,
	profile any, // your TravelProfile or a lightweight struct
	poiList []request_models.POISummary,
	dayCount int,
) (string, error) {

	if dayCount < 1 || dayCount > 30 {
		return "", fmt.Errorf("bad dayCount")
	}
	if len(poiList) == 0 {
		return "", fmt.Errorf("no pois")
	}

	m := c.client.GenerativeModel(c.model)
	// Force JSON-only so you can delete brace-matching hacks:
	m.ResponseMIMEType = "application/json"
	m.SetTopP(0.5)
	m.SetTopK(20)
	m.SetTemperature(0.1)

	schema := `
{
  "destination": "string",
  "duration_days": 3,
  "days": [
    {
      "day": 1,
      "activities": [
        {"start_time":"09:00","end_time":"11:00","main_poi_id":"<ID from list>"}
      ]
    }
  ]
}`

	// Build a tight instruction. No prose, exact JSON keys.
	var poiBuf strings.Builder
	for _, p := range poiList {
		fmt.Fprintf(&poiBuf, "- ID:%s | Name:%s | Category:%s | Description:%s \n", p.ID, p.Name, p.Category, p.Description)
	}

	prompt := fmt.Sprintf(`
You are scheduling a %d-day travel plan. Return **JSON only** that exactly matches the schema below. 
Use only POI IDs from the list. Ensure realistic times (09:00–21:00), 2–5 activities/day, and do not overlap times.
Respect a relaxed pace if the profile indicates "relaxed", otherwise standard.

Schema (example, match keys exactly):
%s

Profile (read-only, use to bias selection and density):
%+v

Allowed POIs (use IDs from here only):
%s

Hard constraints:
- Exactly %d days in "days".
- Each day.day = 1..%d (no gaps).
- start_time < end_time; times formatted HH:MM.
- Choose diverse categories when possible.

Return JSON only. No comments, no markdown.
`, dayCount, schema, profile, poiBuf.String(), dayCount, dayCount)

	resp, err := m.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content")
	}
	content := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	// Because ResponseMIMEType="application/json", this should already be clean JSON.
	if !json.Valid([]byte(content)) {
		return "", fmt.Errorf("not valid json")
	}
	return content, nil
}

// GetEmbedding generates a simple vector embedding for text
// Note: This is a fallback since Gemini free tier doesn't have dedicated embeddings
func (c *GeminiEmbeddingClient) GetEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	// Simple text-to-vector conversion using hash-based approach
	// This is a basic implementation - for production, consider using a proper embedding model
	return c.textToVector(text), nil
}

// GetEmbeddings batch processes multiple texts
func (c *GeminiEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no input texts provided")
	}

	vectors := make([]pgvector.Vector, len(texts))
	for i, text := range texts {
		vectors[i] = c.textToVector(text)
	}

	return vectors, nil
}

// GenerateStructuredPlan uses Gemini to create travel itineraries with optimizations
func (c *GeminiEmbeddingClient) GenerateStructuredPlan(ctx context.Context, userPrompt string, pois []string, dayCount int) (string, error) {
	// Input validation (keep existing validation)
	if strings.TrimSpace(userPrompt) == "" {
		return "", fmt.Errorf("user prompt cannot be empty")
	}
	if len(pois) == 0 {
		return "", fmt.Errorf("POI list cannot be empty")
	}
	if dayCount < 1 {
		return "", fmt.Errorf("day count must be at least 1")
	}
	if dayCount > 30 {
		return "", fmt.Errorf("day count cannot exceed 30 days")
	}

	model := c.client.GenerativeModel(c.model)

	// OPTIMIZATION 1: More aggressive model settings for faster generation
	model.SetTemperature(0.1)      // Lower temperature = faster, more deterministic
	model.SetTopP(0.5)             // Reduced from 0.8 for faster token selection
	model.SetTopK(10)              // Reduced from 20 for faster processing
	model.SetMaxOutputTokens(5000) // Limit output length for faster generation

	// OPTIMIZATION 2: Limit POI list to essential information only
	// Instead of sending full POI descriptions, send only essential data
	limitedPOIs := c.limitPOIData(pois, 10) // Limit to top 10 most relevant POIs

	// OPTIMIZATION 3: Use more concise, structured prompts
	prompt := c.buildOptimizedPrompt(userPrompt, limitedPOIs, dayCount)

	// OPTIMIZATION 4: Single attempt with timeout instead of multiple retries
	// Set a reasonable timeout for the API call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := model.GenerateContent(ctxWithTimeout, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini API call failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated by Gemini")
	}

	// Extract content
	content := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	content = c.cleanJSONResponse(content)

	// OPTIMIZATION 5: Simplified validation - only check basic JSON structure
	if err := c.quickValidateJSON(content, dayCount); err != nil {
		return "", fmt.Errorf("invalid JSON structure: %w", err)
	}

	return content, nil
}

// limitPOIData reduces POI data to essential information only
func (c *GeminiEmbeddingClient) limitPOIData(pois []string, maxPOIs int) []string {
	if len(pois) <= maxPOIs {
		return pois
	}

	// Take only the first maxPOIs (they should already be ranked by relevance)
	limited := make([]string, maxPOIs)
	for i := 0; i < maxPOIs; i++ {
		// Extract only essential info: ID, Name, Brief Description
		limited[i] = c.extractEssentialPOIInfo(pois[i])
	}
	return limited
}

// extractEssentialPOIInfo extracts only essential POI information
func (c *GeminiEmbeddingClient) extractEssentialPOIInfo(poiText string) string {
	// Parse the POI text and extract only ID, Name, and short description
	parts := strings.Split(poiText, "|")
	if len(parts) >= 3 {
		id := strings.TrimSpace(strings.Replace(parts[0], "POI_ID:", "", 1))
		name := strings.TrimSpace(strings.Replace(parts[1], "Name:", "", 1))
		desc := strings.TrimSpace(strings.Replace(parts[2], "Description:", "", 1))

		// Limit description length
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}

		return fmt.Sprintf("ID:%s|Name:%s|Desc:%s", id, name, desc)
	}
	return poiText
}

// buildOptimizedPrompt creates a more concise, faster-to-process prompt
func (c *GeminiEmbeddingClient) buildOptimizedPrompt(userPrompt string, pois []string, dayCount int) string {
	var prompt strings.Builder

	// Much more concise system prompt
	if dayCount > 1 {
		prompt.WriteString(fmt.Sprintf("Create %d-day travel plan. Return JSON only:\n", dayCount))
		prompt.WriteString(`{"days":[{"day":1,"activities":[{"activity":"...","start_time":"09:00","end_time":"11:00","main_poi":{"id":"poi_id","name":"POI Name","description":"desc","province_id":"","category_id":"","tags":[]},"alternatives":[],"what_to_do":"..."}]}]}`)
	} else {
		prompt.WriteString("Create 1-day travel plan. Return JSON array only:\n")
		prompt.WriteString(`[{"activity":"...","start_time":"09:00","end_time":"11:00","main_poi":{"id":"poi_id","name":"POI Name","description":"desc","province_id":"","category_id":"","tags":[]},"alternatives":[],"what_to_do":"..."}]`)
	}

	prompt.WriteString("\n\nPOIs:\n")
	for _, poi := range pois {
		prompt.WriteString(poi + "\n")
	}

	prompt.WriteString(fmt.Sprintf("\nUser: %s\nGenerate plan using POI IDs above. JSON only:", userPrompt))

	return prompt.String()
}

// quickValidateJSON performs minimal validation for faster processing
func (c *GeminiEmbeddingClient) quickValidateJSON(content string, expectedDays int) error {
	// Just check if it's valid JSON and has the right structure
	if !json.Valid([]byte(content)) {
		return fmt.Errorf("invalid JSON")
	}

	// Quick structure check
	if expectedDays > 1 {
		// Check for "days" key
		if !strings.Contains(content, `"days"`) {
			return fmt.Errorf("multi-day format missing 'days' key")
		}
	} else {
		// Should start with [ for single day
		trimmed := strings.TrimSpace(content)
		if !strings.HasPrefix(trimmed, "[") {
			return fmt.Errorf("single-day format should be array")
		}
	}

	return nil
}

// OPTIMIZATION 6: Add caching mechanism
type PlanCache struct {
	plans map[string]CachedPlan
	mutex sync.RWMutex
}

type CachedPlan struct {
	Content   string
	Timestamp time.Time
	DayCount  int
}

var planCache = &PlanCache{
	plans: make(map[string]CachedPlan),
}

// generateCacheKey creates a cache key from the request parameters
func (c *GeminiEmbeddingClient) generateCacheKey(userPrompt string, pois []string, dayCount int) string {
	h := sha256.New()
	h.Write([]byte(userPrompt))
	h.Write([]byte(fmt.Sprintf("%d", dayCount)))
	for _, poi := range pois {
		h.Write([]byte(poi))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16] // Use first 16 characters
}

// getCachedPlan retrieves a cached plan if available and not expired
func (c *GeminiEmbeddingClient) getCachedPlan(cacheKey string) (string, bool) {
	planCache.mutex.RLock()
	defer planCache.mutex.RUnlock()

	cached, exists := planCache.plans[cacheKey]
	if !exists {
		return "", false
	}

	// Cache for 1 hour
	if time.Since(cached.Timestamp) > time.Hour {
		return "", false
	}

	return cached.Content, true
}

// setCachedPlan stores a plan in cache
func (c *GeminiEmbeddingClient) setCachedPlan(cacheKey, content string, dayCount int) {
	planCache.mutex.Lock()
	defer planCache.mutex.Unlock()

	planCache.plans[cacheKey] = CachedPlan{
		Content:   content,
		Timestamp: time.Now(),
		DayCount:  dayCount,
	}

	// Simple cleanup: remove old entries if cache gets too large
	if len(planCache.plans) > 1000 {
		// Remove entries older than 2 hours
		for key, cached := range planCache.plans {
			if time.Since(cached.Timestamp) > 2*time.Hour {
				delete(planCache.plans, key)
			}
		}
	}
}

// GenerateStructuredPlanWithCache - Enhanced version with caching
func (c *GeminiEmbeddingClient) GenerateStructuredPlanWithCache(ctx context.Context, userPrompt string, pois []string, dayCount int) (string, error) {
	// Check cache first
	cacheKey := c.generateCacheKey(userPrompt, pois, dayCount)
	if cached, found := c.getCachedPlan(cacheKey); found {
		log.Printf("Cache hit for travel plan generation")
		return cached, nil
	}

	// Generate new plan
	content, err := c.GenerateStructuredPlan(ctx, userPrompt, pois, dayCount)
	if err != nil {
		return "", err
	}

	// Cache the result
	c.setCachedPlan(cacheKey, content, dayCount)

	return content, nil
}

// validatePlanJSON performs comprehensive validation of the generated travel plan JSON
// validatePlanJSON performs comprehensive validation of the generated travel plan JSON
func (c *GeminiEmbeddingClient) validatePlanJSON(content string, expectedDays int) error {
	// Basic JSON validity check
	if !json.Valid([]byte(content)) {
		return fmt.Errorf("invalid JSON format")
	}

	// Parse and validate structure
	if expectedDays > 1 {
		var plan struct {
			Days []struct {
				Day        int    `json:"day"`
				Date       string `json:"date,omitempty"`
				Activities []struct {
					Activity  string `json:"activity"`
					StartTime string `json:"start_time"`
					EndTime   string `json:"end_time"`
					MainPOI   struct {
						ID          string   `json:"id"`
						Name        string   `json:"name"`
						Description string   `json:"description"`
						ProvinceID  string   `json:"province_id"`
						CategoryID  string   `json:"category_id"`
						Tags        []string `json:"tags"`
					} `json:"main_poi"`
					Alternatives []struct {
						ID          string   `json:"id"`
						Name        string   `json:"name"`
						Description string   `json:"description"`
						ProvinceID  string   `json:"province_id"`
						CategoryID  string   `json:"category_id"`
						Tags        []string `json:"tags"`
					} `json:"alternatives,omitempty"`
					WhatToDo string `json:"what_to_do"`
				} `json:"activities"`
			} `json:"days"`
		}

		if err := json.Unmarshal([]byte(content), &plan); err != nil {
			return fmt.Errorf("failed to unmarshal multi-day plan: %w", err)
		}

		if len(plan.Days) == 0 {
			return fmt.Errorf("plan contains no days")
		}

		if len(plan.Days) != expectedDays {
			return fmt.Errorf("expected %d days, got %d", expectedDays, len(plan.Days))
		}

		// Validate each day
		for i, day := range plan.Days {
			if day.Day != i+1 {
				return fmt.Errorf("day %d has incorrect day number: %d", i+1, day.Day)
			}
			if len(day.Activities) == 0 {
				return fmt.Errorf("day %d has no activities", day.Day)
			}
			for j, activity := range day.Activities {
				if err := c.validateActivityWithPOI(activity.Activity, activity.StartTime, activity.EndTime, activity.MainPOI.ID, activity.WhatToDo); err != nil {
					return fmt.Errorf("day %d, activity %d: %w", day.Day, j+1, err)
				}
			}
		}
	} else {
		var activities []struct {
			Activity  string `json:"activity"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			MainPOI   struct {
				ID          string   `json:"id"`
				Name        string   `json:"name"`
				Description string   `json:"description"`
				ProvinceID  string   `json:"province_id"`
				CategoryID  string   `json:"category_id"`
				Tags        []string `json:"tags"`
			} `json:"main_poi"`
			Alternatives []struct {
				ID          string   `json:"id"`
				Name        string   `json:"name"`
				Description string   `json:"description"`
				ProvinceID  string   `json:"province_id"`
				CategoryID  string   `json:"category_id"`
				Tags        []string `json:"tags"`
			} `json:"alternatives,omitempty"`
			WhatToDo string `json:"what_to_do"`
		}

		if err := json.Unmarshal([]byte(content), &activities); err != nil {
			return fmt.Errorf("failed to unmarshal single-day plan: %w", err)
		}

		if len(activities) == 0 {
			return fmt.Errorf("plan contains no activities")
		}

		for i, activity := range activities {
			if err := c.validateActivityWithPOI(activity.Activity, activity.StartTime, activity.EndTime, activity.MainPOI.ID, activity.WhatToDo); err != nil {
				return fmt.Errorf("activity %d: %w", i+1, err)
			}
		}
	}

	return nil
}

// validateActivityWithPOI validates individual activity fields with POI structure
func (c *GeminiEmbeddingClient) validateActivityWithPOI(activity, startTime, endTime, mainPOIID, whatToDo string) error {
	if strings.TrimSpace(activity) == "" {
		return fmt.Errorf("activity name cannot be empty")
	}
	if strings.TrimSpace(startTime) == "" {
		return fmt.Errorf("start_time cannot be empty")
	}
	if strings.TrimSpace(endTime) == "" {
		return fmt.Errorf("end_time cannot be empty")
	}
	if strings.TrimSpace(mainPOIID) == "" {
		return fmt.Errorf("main_poi.id cannot be empty")
	}
	if strings.TrimSpace(whatToDo) == "" {
		return fmt.Errorf("what_to_do cannot be empty")
	}

	// Validate time format (HH:MM)
	timePattern := `^([01]?[0-9]|2[0-3]):[0-5][0-9]$`
	if matched, _ := regexp.MatchString(timePattern, startTime); !matched {
		return fmt.Errorf("invalid start_time format: %s (expected HH:MM)", startTime)
	}
	if matched, _ := regexp.MatchString(timePattern, endTime); !matched {
		return fmt.Errorf("invalid end_time format: %s (expected HH:MM)", endTime)
	}

	return nil
}

// validateActivity validates individual activity fields
func (c *GeminiEmbeddingClient) validateActivity(activity, startTime, endTime, mainPOIID, whatToDo string) error {
	if strings.TrimSpace(activity) == "" {
		return fmt.Errorf("activity name cannot be empty")
	}
	if strings.TrimSpace(startTime) == "" {
		return fmt.Errorf("start_time cannot be empty")
	}
	if strings.TrimSpace(endTime) == "" {
		return fmt.Errorf("end_time cannot be empty")
	}
	if strings.TrimSpace(mainPOIID) == "" {
		return fmt.Errorf("main_poi_id cannot be empty")
	}
	if strings.TrimSpace(whatToDo) == "" {
		return fmt.Errorf("what_to_do cannot be empty")
	}

	// Validate time format (HH:MM)
	timePattern := `^([01]?[0-9]|2[0-3]):[0-5][0-9]$`
	if matched, _ := regexp.MatchString(timePattern, startTime); !matched {
		return fmt.Errorf("invalid start_time format: %s (expected HH:MM)", startTime)
	}
	if matched, _ := regexp.MatchString(timePattern, endTime); !matched {
		return fmt.Errorf("invalid end_time format: %s (expected HH:MM)", endTime)
	}

	return nil
}

// cleanJSONResponse removes markdown formatting and extra text with improved extraction
func (c *GeminiEmbeddingClient) cleanJSONResponse(response string) string {
	// Remove markdown code blocks
	response = strings.ReplaceAll(response, "```json", "")
	response = strings.ReplaceAll(response, "```JSON", "")
	response = strings.ReplaceAll(response, "```", "")

	// Remove common prefixes that LLMs might add
	prefixes := []string{
		"Here's the travel plan:",
		"Here is the itinerary:",
		"The travel plan is:",
		"Travel plan:",
		"Itinerary:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.TrimSpace(response), prefix) {
			response = strings.TrimPrefix(response, prefix)
			break
		}
	}

	// Find JSON boundaries more accurately
	response = strings.TrimSpace(response)

	// Look for object start
	objStart := strings.Index(response, "{")
	arrStart := strings.Index(response, "[")

	if objStart != -1 && (arrStart == -1 || objStart < arrStart) {
		// It's an object - find matching closing brace
		objEnd := c.findMatchingBrace(response, objStart)
		if objEnd != -1 {
			response = response[objStart : objEnd+1]
		}
	} else if arrStart != -1 {
		// It's an array - find matching closing bracket
		arrEnd := c.findMatchingBracket(response, arrStart)
		if arrEnd != -1 {
			response = response[arrStart : arrEnd+1]
		}
	}

	return strings.TrimSpace(response)
}

// findMatchingBrace finds the matching closing brace for an opening brace
func (c *GeminiEmbeddingClient) findMatchingBrace(s string, start int) int {
	if start >= len(s) || s[start] != '{' {
		return -1
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		char := s[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' && inString {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// findMatchingBracket finds the matching closing bracket for an opening bracket
func (c *GeminiEmbeddingClient) findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		char := s[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' && inString {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch char {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// textToVector creates a simple vector representation of text
// This is a basic hash-based approach for demonstration
// For production use, consider integrating with Sentence Transformers or similar
func (c *GeminiEmbeddingClient) textToVector(text string) pgvector.Vector {
	// Normalize text
	text = strings.ToLower(strings.TrimSpace(text))
	words := strings.Fields(text)

	// Create a 384-dimensional vector (common embedding size)
	const dimensions = 1536
	vector := make([]float32, dimensions)

	// Use word hashing to populate vector
	for _, word := range words {
		hash := c.hashWord(word)
		for i := 0; i < dimensions; i++ {
			// Distribute word influence across dimensions
			influence := math.Sin(float64(hash+uint32(i))) * 0.1
			vector[i] += float32(influence)
		}
	}

	// Normalize the vector
	magnitude := float32(0)
	for _, val := range vector {
		magnitude += val * val
	}
	magnitude = float32(math.Sqrt(float64(magnitude)))

	if magnitude > 0 {
		for i := range vector {
			vector[i] /= magnitude
		}
	}

	return pgvector.NewVector(vector)
}

// hashWord creates a hash for a word
func (c *GeminiEmbeddingClient) hashWord(word string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(word))
	return h.Sum32()
}

// Close closes the Gemini client
func (c *GeminiEmbeddingClient) Close() error {
	return c.client.Close()
}

// NewEmbeddingClient Factory function to create either OpenAI or Gemini client based on config
func NewEmbeddingClient(provider, apiKey, model string) (EmbeddingClientInterface, error) {
	switch strings.ToLower(provider) {
	case "openai":
		return &OpenAIEmbeddingClient{
			client: openai.NewClient(apiKey),
			model:  model,
		}, nil
	case "gemini":
		return NewGeminiEmbeddingClient(apiKey, model)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
