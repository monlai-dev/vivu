package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type PromptServiceInterface interface {
	CreatePrompt(ctx context.Context, prompt string) (string, error)
	PromptInput(ctx context.Context, request request_models.CreateTagRequest) (string, error)
	CreateAIPlan(ctx context.Context, userPrompt string) ([]response_models.DailyPlan, error)
	ExtractLocationFromPrompt(prompt string) []string
}

type PromptService struct {
	poisService POIServiceInterface
	tagService  TagServiceInterface
	aiService   utils.EmbeddingClientInterface
	embededRepo repositories.IPoiEmbededRepository
	poisRepo    repositories.POIRepository
}

func NewPromptService(
	poisService POIServiceInterface,
	tagService TagServiceInterface,
	aiService utils.EmbeddingClientInterface,
	embededRepo repositories.IPoiEmbededRepository,
	poisRepo repositories.POIRepository,
) PromptServiceInterface {
	return &PromptService{
		poisService: poisService,
		tagService:  tagService,
		aiService:   aiService,
		embededRepo: embededRepo,
		poisRepo:    poisRepo,
	}
}

func (p *PromptService) ExtractLocationFromPrompt(prompt string) []string {
	var locations []string
	lower := strings.ToLower(prompt)

	// Common location patterns for English and Vietnamese
	locationPatterns := []string{
		// English patterns
		`to\s+([A-Za-z\s]+?)(?:\s+in|\s+for|\s+during|\s+\d|$)`,
		`in\s+([A-Za-z\s]+?)(?:\s+for|\s+during|\s+\d|$)`,
		`visit\s+([A-Za-z\s]+?)(?:\s+in|\s+for|\s+during|\s+\d|$)`,
		`around\s+([A-Za-z\s]+?)(?:\s+in|\s+for|\s+during|\s+\d|$)`,

		// Vietnamese patterns
		`đến\s+([A-Za-zÀ-ỹ\s]+?)(?:\s+trong|\s+cho|\s+vào|\s+\d|$)`,
		`ở\s+([A-Za-zÀ-ỹ\s]+?)(?:\s+trong|\s+cho|\s+vào|\s+\d|$)`,
		`thăm\s+([A-Za-zÀ-ỹ\s]+?)(?:\s+trong|\s+cho|\s+vào|\s+\d|$)`,
	}

	for _, pattern := range locationPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(lower, -1)
		for _, match := range matches {
			if len(match) > 1 {
				location := strings.TrimSpace(match[1])
				if len(location) > 2 && len(location) < 50 {
					locations = append(locations, location)
				}
			}
		}
	}

	// Check for well-known Vietnamese locations
	knownLocations := []string{
		"da lat", "dalat", "đà lạt",
		"ho chi minh", "hồ chí minh", "saigon", "sài gòn",
		"ha noi", "hanoi", "hà nội",
		"hoi an", "hội an",
		"nha trang",
		"phu quoc", "phú quốc",
		"ha long", "hạ long",
		"sapa", "sa pa",
		"mui ne", "mũi né",
		"can tho", "cần thơ",
		"hue", "huế",
		"vung tau", "vũng tàu",
	}

	for _, location := range knownLocations {
		if strings.Contains(lower, location) {
			locations = append(locations, location)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueLocations []string
	for _, loc := range locations {
		if !seen[loc] {
			seen[loc] = true
			uniqueLocations = append(uniqueLocations, loc)
		}
	}

	return uniqueLocations
}

func (p *PromptService) CreatePrompt(ctx context.Context, prompt string) (string, error) {
	// Get embedding for the prompt
	vector, err := p.aiService.GetEmbedding(ctx, prompt)
	if err != nil {
		return "", utils.ErrUnexpectedBehaviorOfAI
	}

	log.Printf("Creating prompt with vector: %v", vector)

	// Get similar POIs based on vector similarity
	poiEmbeddedIds, err := p.embededRepo.GetListOfPoiEmbededByVector(vector, nil)
	if err != nil {
		return "", utils.ErrDatabaseError
	}

	if len(poiEmbeddedIds) == 0 {
		return "No relevant places found for your request.", nil
	}

	// Extract POI IDs
	var poiIdList []string
	for _, poiEmbedded := range poiEmbeddedIds {
		poiIdList = append(poiIdList, poiEmbedded.PoiID)
	}

	// Get POI details
	pois, err := p.poisRepo.ListPoisByPoisId(ctx, poiIdList)
	if err != nil {
		return "", utils.ErrPOINotFound
	}

	// Format response
	var responseBuilder strings.Builder
	responseBuilder.WriteString("Here are some relevant places based on your request:\n\n")

	for i, poi := range pois {
		responseBuilder.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, poi.Name))
		responseBuilder.WriteString(fmt.Sprintf("   - %s\n", poi.Description))
		if poi.Address != "" {
			responseBuilder.WriteString(fmt.Sprintf("   - Address: %s\n", poi.Address))
		}
		if poi.OpeningHours != "" {
			responseBuilder.WriteString(fmt.Sprintf("   - Hours: %s\n", poi.OpeningHours))
		}
		responseBuilder.WriteString("\n")
	}

	return responseBuilder.String(), nil
}

func (p *PromptService) PromptInput(ctx context.Context, request request_models.CreateTagRequest) (string, error) {
	// Create a search prompt based on the tag
	if request.En == "" && request.Vi == "" {
		return "", utils.ErrPoorQualityInput
	}

	searchPrompt := fmt.Sprintf("Find places related to %s", request.En)
	if request.Vi != "" {
		searchPrompt += fmt.Sprintf(" (%s)", request.Vi)
	}

	return p.CreatePrompt(ctx, searchPrompt)
}

func (p *PromptService) CreateAIPlan(ctx context.Context, userPrompt string) ([]response_models.DailyPlan, error) {
	// Validate input
	if strings.TrimSpace(userPrompt) == "" {
		return nil, utils.ErrInvalidInput
	}

	startTime := time.Now()
	log.Printf("ts: %d - Creating AI plan for prompt: %s", time.Since(startTime), userPrompt)

	// Find POIs using multiple strategies
	pois, err := p.findRelevantPOIs(ctx, userPrompt)
	if err != nil {
		return nil, utils.ErrPOINotFound
	}

	log.Printf("ts: %d - Complete related pois ", time.Since(startTime))
	log.Printf("Found %d relevant POIs for user prompt", len(pois))

	if len(pois) == 0 {
		return nil, utils.ErrPoorQualityInput
	}

	// Prepare POI data for AI
	var poiTextList []string
	poiMap := make(map[string]response_models.ActivityPOI)
	poiNameToID := make(map[string]string)

	for _, poi := range pois {
		poiText := fmt.Sprintf("POI_ID: %s | Name: %s | Description: %s",
			poi.ID.String(), poi.Name, poi.Description)
		if poi.Address != "" {
			poiText += fmt.Sprintf(" | Address: %s", poi.Address)
		}
		if poi.OpeningHours != "" {
			poiText += fmt.Sprintf(" | Hours: %s", poi.OpeningHours)
		}

		poiTextList = append(poiTextList, poiText)
		poiID := poi.ID.String()
		poiMap[poiID] = response_models.ActivityPOI{
			ID:          poiID,
			Name:        poi.Name,
			Description: poi.Description,
			ProvinceID:  poi.ProvinceID.String(),
			CategoryID:  poi.CategoryID.String(),
			Tags:        flattenTags(poi.Tags),
		}
		poiNameToID[strings.ToLower(poi.Name)] = poiID
	}

	dayCount := extractDayCount(userPrompt)
	log.Printf("Planning for %d day(s) with %d POIs", dayCount, len(pois))

	// Generate AI plan with improved prompting
	rawJSON, err := p.generateAIPlanWithRetry(ctx, userPrompt, poiTextList, dayCount)
	if err != nil {
		log.Printf("AI generation error: %v", err)
		return nil, utils.ErrUnexpectedBehaviorOfAI
	}

	log.Printf("Raw AI JSON response: %s", rawJSON)

	// Parse response and convert to DailyPlan format
	activityBlocks, err := p.parseAndConvertToActivityBlocks(rawJSON, dayCount, poiMap, poiNameToID)
	if err != nil {
		return nil, err
	}

	// Convert activity blocks to daily plans
	return p.convertToDailyPlans(activityBlocks, dayCount, userPrompt), nil
}

// Parse response based on day count and convert to activity blocks
func (p *PromptService) parseAndConvertToActivityBlocks(rawJSON string, dayCount int, poiMap map[string]response_models.ActivityPOI, poiNameToID map[string]string) ([]response_models.ActivityPlanBlock, error) {
	if dayCount > 1 {
		return p.parseMultiDayPlanImproved(rawJSON, poiMap, poiNameToID)
	}
	return p.parseSingleDayPlanImproved(rawJSON, poiMap, poiNameToID)
}

// Convert activity blocks to daily plans
func (p *PromptService) convertToDailyPlans(activityBlocks []response_models.ActivityPlanBlock, dayCount int, userPrompt string) []response_models.DailyPlan {
	var dailyPlans []response_models.DailyPlan

	// Extract locations for potential accommodation suggestions
	//locations := p.ExtractLocationFromPrompt(userPrompt)
	//baseLocation := "local area"
	//if len(locations) > 0 {
	//	baseLocation = locations[0]
	//}

	if dayCount == 1 {
		// Single day plan
		dailyPlan := response_models.DailyPlan{
			Day:        1,
			Date:       time.Now().Format("2006-01-02"),
			Activities: activityBlocks,
		}
		dailyPlans = append(dailyPlans, dailyPlan)
		return dailyPlans
	}

	// Multi-day plan - organize activities by day
	currentDay := 1
	var currentActivities []response_models.ActivityPlanBlock

	for _, block := range activityBlocks {
		// Check if this is a day separator
		if strings.HasPrefix(block.Activity, "Day ") {
			// Save previous day if it has activities
			if len(currentActivities) > 0 {
				dailyPlan := response_models.DailyPlan{
					Day:  currentDay,
					Date: time.Now().AddDate(0, 0, currentDay-1).Format("2006-01-02"),
					//Stay:       p.generateStayOption(baseLocation, currentDay),
					Activities: currentActivities,
				}
				dailyPlans = append(dailyPlans, dailyPlan)
				currentActivities = []response_models.ActivityPlanBlock{}
			}

			// Extract day number from the separator
			if dayNum := p.extractDayNumber(block.Activity); dayNum > 0 {
				currentDay = dayNum
			}
			continue
		}

		// Add regular activities
		currentActivities = append(currentActivities, block)
	}

	// Add the last day
	if len(currentActivities) > 0 {
		dailyPlan := response_models.DailyPlan{
			Day:  currentDay,
			Date: time.Now().AddDate(0, 0, currentDay-1).Format("2006-01-02"),
			//Stay:       p.generateStayOption(baseLocation, currentDay),
			Activities: currentActivities,
		}
		dailyPlans = append(dailyPlans, dailyPlan)
	}

	// Ensure we have the expected number of days
	for len(dailyPlans) < dayCount {
		dayNum := len(dailyPlans) + 1
		dailyPlan := response_models.DailyPlan{
			Day:  dayNum,
			Date: time.Now().AddDate(0, 0, dayNum-1).Format("2006-01-02"),
			//Stay:   p.generateStayOption(baseLocation, dayNum),
			Activities: []response_models.ActivityPlanBlock{
				{
					Activity:  "Free time",
					StartTime: "09:00",
					EndTime:   "18:00",
					MainPOI: response_models.ActivityPOI{
						Name:        "Free exploration",
						Description: "Explore the area at your own pace",
					},
					WhatToDo: "Use this time to explore based on your interests",
				},
			},
		}
		dailyPlans = append(dailyPlans, dailyPlan)
	}

	return dailyPlans
}

// Extract day number from day separator text
func (p *PromptService) extractDayNumber(dayText string) int {
	// Extract number from "Day X" format
	re := regexp.MustCompile(`Day (\d+)`)
	matches := re.FindStringSubmatch(dayText)
	if len(matches) >= 2 {
		if dayNum, err := strconv.Atoi(matches[1]); err == nil {
			return dayNum
		}
	}
	return 0
}

// Create structured prompt based on day count
func (p *PromptService) createStructuredPrompt(userPrompt string, poiTextList []string, dayCount int) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Create a detailed %d-day travel itinerary.\n\n", dayCount))
	prompt.WriteString("Available POIs:\n")
	for _, poi := range poiTextList {
		prompt.WriteString(fmt.Sprintf("- %s\n", poi))
	}

	prompt.WriteString(fmt.Sprintf("\nUser Request: %s\n\n", userPrompt))

	prompt.WriteString("CRITICAL REQUIREMENTS:\n")
	prompt.WriteString(fmt.Sprintf("1. Generate exactly %d days of activities\n", dayCount))
	prompt.WriteString("2. Use exact POI_ID values from the list above\n")
	prompt.WriteString("3. Each activity must have a main_poi_id from the provided list\n")
	prompt.WriteString("4. Provide 1-2 alternative_poi_ids for each activity\n")
	prompt.WriteString("5. Use realistic time slots (e.g., 09:00, 14:30)\n")
	prompt.WriteString("6. Return ONLY valid JSON, no extra text\n\n")

	if dayCount == 1 {
		prompt.WriteString("Return JSON in this EXACT format:\n")
		prompt.WriteString(`[
  {
    "activity": "Visit [POI Name]",
    "start_time": "09:00",
    "end_time": "11:30",
    "main_poi_id": "exact-poi-id-from-list",
    "alternative_poi_ids": ["alternative-poi-id-1", "alternative-poi-id-2"],
    "what_to_do": "Detailed description of activities"
  },
  {
    "activity": "Lunch at [Restaurant Name]",
    "start_time": "12:00", 
    "end_time": "13:30",
    "main_poi_id": "restaurant-poi-id",
    "alternative_poi_ids": ["alternative-restaurant-id"],
    "what_to_do": "Enjoy local cuisine"
  }
]`)
	} else {
		prompt.WriteString("Return JSON in this EXACT format:\n")
		prompt.WriteString(`{
  "days": [
    {
      "day": 1,
      "date": "2024-01-01",
      "activities": [
        {
          "activity": "Morning visit to [POI Name]",
          "start_time": "09:00",
          "end_time": "11:30",
          "main_poi_id": "exact-poi-id-from-list",
          "alternative_poi_ids": ["alternative-poi-id-1"],
          "what_to_do": "Detailed description"
        }
      ]
    },
    {
      "day": 2, 
      "date": "2024-01-02",
      "activities": [
        {
          "activity": "Visit [Another POI]",
          "start_time": "09:00",
          "end_time": "11:30", 
          "main_poi_id": "another-exact-poi-id",
          "alternative_poi_ids": ["alternative-id"],
          "what_to_do": "Description of activities"
        }
      ]
    }
  ]
}`)
	}

	return prompt.String()
}

// Validate JSON structure matches expected day count
func (p *PromptService) validateJSONStructure(rawJSON string, expectedDays int) bool {
	if expectedDays == 1 {
		// For single day, expect an array
		var singleDay []interface{}
		if err := json.Unmarshal([]byte(rawJSON), &singleDay); err != nil {
			log.Printf("Single day JSON validation failed: %v", err)
			return false
		}
		return len(singleDay) > 0
	} else {
		// For multi-day, expect days object
		var multiDay struct {
			Days []struct {
				Day int `json:"day"`
			} `json:"days"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &multiDay); err != nil {
			log.Printf("Multi-day JSON validation failed: %v", err)
			return false
		}

		if len(multiDay.Days) != expectedDays {
			log.Printf("Expected %d days, got %d days", expectedDays, len(multiDay.Days))
			return false
		}

		return true
	}
}

// Improved multi-day plan parser with better error handling
func (p *PromptService) parseMultiDayPlanImproved(rawJSON string, poiMap map[string]response_models.ActivityPOI, poiNameToID map[string]string) ([]response_models.ActivityPlanBlock, error) {
	// First try to fix common JSON issues
	rawJSON = p.cleanAndFixJSON(rawJSON)

	var multiDayPlan struct {
		Days []struct {
			Day        int    `json:"day"`
			Date       string `json:"date,omitempty"`
			Activities []struct {
				Activity          string   `json:"activity"`
				StartTime         string   `json:"start_time"`
				EndTime           string   `json:"end_time"`
				MainPoiID         string   `json:"main_poi_id"`
				AlternativePoiIDs []string `json:"alternative_poi_ids"`
				WhatToDo          string   `json:"what_to_do"`
			} `json:"activities"`
		} `json:"days"`
	}

	if err := json.Unmarshal([]byte(rawJSON), &multiDayPlan); err != nil {
		log.Printf("Failed to parse multi-day JSON: %v", err)
		log.Printf("Raw JSON: %s", rawJSON)
		return nil, fmt.Errorf("invalid multi-day plan JSON: %w", err)
	}

	if len(multiDayPlan.Days) == 0 {
		return nil, fmt.Errorf("no days found in multi-day plan")
	}

	var allActivities []response_models.ActivityPlanBlock

	for _, day := range multiDayPlan.Days {
		// Add day separator
		dayHeader := response_models.ActivityPlanBlock{
			Activity:  fmt.Sprintf("Day %d", day.Day),
			StartTime: "00:00",
			EndTime:   "23:59",
			MainPOI: response_models.ActivityPOI{
				ID:          "",
				Name:        fmt.Sprintf("Day %d Overview", day.Day),
				Description: "Day planning overview",
			},
			WhatToDo: fmt.Sprintf("Activities for day %d", day.Day),
		}
		if day.Date != "" {
			dayHeader.WhatToDo += fmt.Sprintf(" (%s)", day.Date)
		}
		allActivities = append(allActivities, dayHeader)

		// Add activities for this day
		if len(day.Activities) > 0 {
			dayActivities := p.buildActivityBlocksImproved(day.Activities, poiMap, poiNameToID)
			allActivities = append(allActivities, dayActivities...)
		} else {
			// Add placeholder if no activities
			allActivities = append(allActivities, response_models.ActivityPlanBlock{
				Activity:  "Free time",
				StartTime: "09:00",
				EndTime:   "18:00",
				MainPOI: response_models.ActivityPOI{
					Name:        "Free exploration",
					Description: "Explore the area at your own pace",
				},
				WhatToDo: "Use this time to explore based on your interests",
			})
		}
	}

	return allActivities, nil
}

// Clean and fix common JSON issues
func (p *PromptService) cleanAndFixJSON(rawJSON string) string {
	// Remove any Markdown formatting
	rawJSON = strings.ReplaceAll(rawJSON, "```json", "")
	rawJSON = strings.ReplaceAll(rawJSON, "```", "")

	// Trim whitespace
	rawJSON = strings.TrimSpace(rawJSON)

	// Fix common JSON issues
	rawJSON = strings.ReplaceAll(rawJSON, `"main_poi_id": null`, `"main_poi_id": ""`)
	rawJSON = strings.ReplaceAll(rawJSON, `"alternative_poi_ids": null`, `"alternative_poi_ids": []`)

	return rawJSON
}

// Enhanced day count extraction with better patterns
func extractDayCount(prompt string) int {
	lower := strings.ToLower(prompt)

	// Check for explicit day mentions with more patterns
	for i := 1; i <= 14; i++ {
		patterns := []string{
			fmt.Sprintf("%d days", i),
			fmt.Sprintf("%d ngày", i),
			fmt.Sprintf("%d day", i),
			fmt.Sprintf("%d-day", i),
			fmt.Sprintf("in %d days", i),
			fmt.Sprintf("for %d days", i),
			fmt.Sprintf("%d days in", i),
			fmt.Sprintf("%d days to", i),
		}

		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				log.Printf("Found day pattern: '%s' -> %d days", pattern, i)
				return i
			}
		}
	}

	// Check for written numbers with more context
	writtenNumbers := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"một": 1, "hai": 2, "ba": 3, "bốn": 4, "năm": 5,
		"sáu": 6, "bảy": 7, "tám": 8, "chín": 9, "mười": 10,
	}

	for word, num := range writtenNumbers {
		patterns := []string{
			word + " day",
			word + " ngày",
			"in " + word + " day",
			"for " + word + " day",
		}
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				log.Printf("Found written number pattern: '%s' -> %d days", pattern, num)
				return num
			}
		}
	}

	// Check for weekend/week patterns
	if strings.Contains(lower, "weekend") || strings.Contains(lower, "cuối tuần") {
		log.Printf("Found weekend pattern -> 2 days")
		return 2
	}
	if strings.Contains(lower, "week") || strings.Contains(lower, "tuần") {
		log.Printf("Found week pattern -> 7 days")
		return 7
	}

	log.Printf("No day pattern found, defaulting to 1 day")
	return 1
}

// Add this method to handle AI service calls with better error handling
func (p *PromptService) callAIServiceWithStructuredPrompt(ctx context.Context, userPrompt string, poiTextList []string, dayCount int) (string, error) {
	// Create a very explicit prompt for the AI
	prompt := p.buildExplicitAIPrompt(userPrompt, poiTextList, dayCount)

	log.Printf("Sending structured prompt to AI for %d days", dayCount)
	log.Printf("Prompt: %s", prompt)

	return p.aiService.GenerateStructuredPlan(ctx, prompt, poiTextList, dayCount)
}

// Build very explicit AI prompt
func (p *PromptService) buildExplicitAIPrompt(userPrompt string, poiTextList []string, dayCount int) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("You must create exactly a %d-day travel itinerary. ", dayCount))

	if dayCount > 1 {
		prompt.WriteString("IMPORTANT: Return multi-day format with 'days' array containing exactly %d day objects.\n\n")
	} else {
		prompt.WriteString("IMPORTANT: Return single-day format as an activities array.\n\n")
	}

	prompt.WriteString("Available POIs (use exact POI_ID values):\n")
	for i, poi := range poiTextList {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, poi))
	}

	prompt.WriteString(fmt.Sprintf("\nUser Request: %s\n\n", userPrompt))

	if dayCount > 1 {
		prompt.WriteString(fmt.Sprintf("Return JSON with exactly %d days in this format:\n", dayCount))
		prompt.WriteString(`{
  "days": [`)

		for i := 1; i <= dayCount; i++ {
			if i > 1 {
				prompt.WriteString(`,`)
			}
			prompt.WriteString(fmt.Sprintf(`
    {
      "day": %d,
      "date": "2024-01-%02d",
      "activities": [
        {
          "activity": "Morning activity",
          "start_time": "09:00",
          "end_time": "11:30",
          "main_poi_id": "use-exact-poi-id-from-list",
          "alternative_poi_ids": ["alternative-id-1", "alternative-id-2"],
          "what_to_do": "Detailed description"
        }
      ]
    }`, i, i))
		}

		prompt.WriteString(`
  ]
}`)
	} else {
		prompt.WriteString("Return JSON in this format:\n")
		prompt.WriteString(`[
  {
    "activity": "Activity description",
    "start_time": "09:00",
    "end_time": "11:30",
    "main_poi_id": "use-exact-poi-id-from-list",
    "alternative_poi_ids": ["alternative-id-1"],
    "what_to_do": "What to do here"
  }
]`)
	}

	return prompt.String()
}

// Try to convert single day format to multi-day format
func (p *PromptService) tryConvertSingleToMultiDay(rawJSON string, expectedDays int) bool {
	var activities []interface{}
	if err := json.Unmarshal([]byte(rawJSON), &activities); err != nil {
		return false
	}

	log.Printf("AI returned single-day format, attempting conversion to %d days", expectedDays)

	// Split activities across days
	activitiesPerDay := len(activities) / expectedDays
	if activitiesPerDay == 0 {
		activitiesPerDay = 1
	}

	// This would require modifying the rawJSON string, which is complex
	// For now, just log the issue
	log.Printf("Would need to convert %d activities into %d days", len(activities), expectedDays)
	return false
}

// Enhanced error handling wrapper
func (p *PromptService) generateAIPlanWithRetry(ctx context.Context, userPrompt string, poiTextList []string, dayCount int) (string, error) {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("AI generation attempt %d/%d for %d days", attempt, maxAttempts, dayCount)

		// Create increasingly explicit prompts
		var prompt string
		switch attempt {
		case 1:
			prompt = p.buildExplicitAIPrompt(userPrompt, poiTextList, dayCount)
		case 2:
			//prompt = p.buildVeryExplicitAIPrompt(userPrompt, poiTextList, dayCount)
		case 3:
			prompt = p.buildUltraExplicitAIPrompt(userPrompt, poiTextList, dayCount)
		}

		rawJSON, err := p.aiService.GenerateStructuredPlan(ctx, prompt, poiTextList, dayCount)
		if err != nil {
			log.Printf("Attempt %d failed with AI service error: %v", attempt, err)
			if attempt == maxAttempts {
				return "", err
			}
			continue
		}

		// Clean and validate
		cleanJSON := p.cleanAndFixJSON(rawJSON)
		if p.validateJSONStructure(cleanJSON, dayCount) {
			log.Printf("Valid JSON received on attempt %d", attempt)
			return cleanJSON, nil
		}

		log.Printf("Attempt %d: Invalid structure for %d days", attempt, dayCount)
		log.Printf("Response: %s", rawJSON)

		//// On final attempt, try to salvage what we can
		//if attempt == maxAttempts {
		//	if salvaged := p.salvageResponse(rawJSON, dayCount, poiMap); salvaged != "" {
		//		log.Printf("Salvaged response on final attempt")
		//		return salvaged, nil
		//	}
		//}
	}

	return "", fmt.Errorf("gemini returned invalid JSON after %d attempts: expected %d days, all attempts failed", maxAttempts, dayCount)
}

// Ultra explicit prompt for final attempt
func (p *PromptService) buildUltraExplicitAIPrompt(userPrompt string, poiTextList []string, dayCount int) string {
	var prompt strings.Builder

	prompt.WriteString("=== CRITICAL INSTRUCTIONS ===\n")
	prompt.WriteString(fmt.Sprintf("You MUST create exactly %d days. Do not create %d day or any other number.\n", dayCount, dayCount-1))
	prompt.WriteString("You MUST return valid JSON only. No explanations.\n")
	prompt.WriteString("You MUST use the exact format specified below.\n\n")

	// Add the basic prompt
	prompt.WriteString(p.buildExplicitAIPrompt(userPrompt, poiTextList, dayCount))

	prompt.WriteString(fmt.Sprintf("\n\n=== REMINDER ===\nReturn exactly %d days in the JSON structure above. Nothing else.\n", dayCount))

	return prompt.String()
}

// Salvage response by creating a valid structure from partial data
func (p *PromptService) salvageResponse(rawJSON string, expectedDays int, poiMap map[string]response_models.ActivityPOI) string {
	log.Printf("Attempting to salvage response for %d days", expectedDays)

	// Try to extract any activities we can find
	var activities []interface{}
	if err := json.Unmarshal([]byte(rawJSON), &activities); err == nil && len(activities) > 0 {
		// We got a single-day format, convert to multi-day
		return p.convertSingleToMultiDayJSON(activities, expectedDays)
	}

	// Create minimal valid response
	return p.createFallbackPlan(expectedDays, poiMap)
}

// Convert single day activities to multi-day format
func (p *PromptService) convertSingleToMultiDayJSON(activities []interface{}, expectedDays int) string {
	activitiesPerDay := len(activities) / expectedDays
	if activitiesPerDay == 0 {
		activitiesPerDay = 1
	}

	var result struct {
		Days []struct {
			Day        int           `json:"day"`
			Date       string        `json:"date"`
			Activities []interface{} `json:"activities"`
		} `json:"days"`
	}

	for i := 0; i < expectedDays; i++ {
		start := i * activitiesPerDay
		end := start + activitiesPerDay
		if end > len(activities) {
			end = len(activities)
		}

		dayActivities := activities[start:end]
		if len(dayActivities) == 0 && i == expectedDays-1 {
			// Add remaining activities to last day
			dayActivities = activities[start:]
		}

		result.Days = append(result.Days, struct {
			Day        int           `json:"day"`
			Date       string        `json:"date"`
			Activities []interface{} `json:"activities"`
		}{
			Day:        i + 1,
			Date:       fmt.Sprintf("2024-01-%02d", i+1),
			Activities: dayActivities,
		})
	}

	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Create fallback plan when all else fails
func (p *PromptService) createFallbackPlan(expectedDays int, poiMap map[string]response_models.ActivityPOI) string {
	log.Printf("Creating fallback plan for %d days", expectedDays)

	// Get first few POIs for the fallback
	var availablePOIs []response_models.ActivityPOI
	for _, poi := range poiMap {
		availablePOIs = append(availablePOIs, poi)
		if len(availablePOIs) >= expectedDays*2 { // 2 activities per day
			break
		}
	}

	if expectedDays == 1 {
		// Create single day fallback
		var activities []interface{}
		for i, poi := range availablePOIs {
			if i >= 3 { // Max 3 activities for single day
				break
			}
			activity := map[string]interface{}{
				"activity":            fmt.Sprintf("Visit %s", poi.Name),
				"start_time":          fmt.Sprintf("%02d:00", 9+(i*3)),
				"end_time":            fmt.Sprintf("%02d:30", 11+(i*3)),
				"main_poi_id":         poi.ID,
				"alternative_poi_ids": []string{},
				"what_to_do":          poi.Description,
			}
			activities = append(activities, activity)
		}

		jsonBytes, _ := json.Marshal(activities)
		return string(jsonBytes)
	} else {
		// Create multi-day fallback
		var result struct {
			Days []interface{} `json:"days"`
		}

		for day := 1; day <= expectedDays; day++ {
			var dayActivities []interface{}

			// Add 2 activities per day
			for i := 0; i < 2 && (day-1)*2+i < len(availablePOIs); i++ {
				poi := availablePOIs[(day-1)*2+i]
				activity := map[string]interface{}{
					"activity":            fmt.Sprintf("Visit %s", poi.Name),
					"start_time":          fmt.Sprintf("%02d:00", 9+(i*4)),
					"end_time":            fmt.Sprintf("%02d:30", 12+(i*4)),
					"main_poi_id":         poi.ID,
					"alternative_poi_ids": []string{},
					"what_to_do":          poi.Description,
				}
				dayActivities = append(dayActivities, activity)
			}

			dayObj := map[string]interface{}{
				"day":        day,
				"date":       fmt.Sprintf("2024-01-%02d", day),
				"activities": dayActivities,
			}

			result.Days = append(result.Days, dayObj)
		}

		jsonBytes, _ := json.Marshal(result)
		return string(jsonBytes)
	}
}

// Improved single day plan parser with fallback matching
func (p *PromptService) parseSingleDayPlanImproved(rawJSON string, poiMap map[string]response_models.ActivityPOI, poiNameToID map[string]string) ([]response_models.ActivityPlanBlock, error) {
	var skeleton []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}

	if err := json.Unmarshal([]byte(rawJSON), &skeleton); err != nil {
		return nil, fmt.Errorf("invalid single-day plan JSON: %w", err)
	}

	return p.buildActivityBlocksImproved(skeleton, poiMap, poiNameToID), nil
}

// Improved activity block builder with better POI matching
func (p *PromptService) buildActivityBlocksImproved(activities interface{}, poiMap map[string]response_models.ActivityPOI, poiNameToID map[string]string) []response_models.ActivityPlanBlock {
	var finalPlan []response_models.ActivityPlanBlock

	// Convert activities to the expected format
	var activityList []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}

	// Handle different activity types
	switch v := activities.(type) {
	case []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}:
		activityList = v
	default:
		// Convert via JSON marshaling/unmarshaling
		jsonData, _ := json.Marshal(activities)
		json.Unmarshal(jsonData, &activityList)
	}

	for _, block := range activityList {
		var mainPOI response_models.ActivityPOI
		var found bool

		// Try to find POI by ID first
		if block.MainPoiID != "" {
			mainPOI, found = poiMap[block.MainPoiID]
		}

		// If not found by ID, try to match by name from activity description
		if !found {
			foundID := p.findPOIByNameInActivity(block.Activity, block.WhatToDo, poiNameToID)
			if foundID != "" {
				mainPOI, found = poiMap[foundID]
				log.Printf("Found POI by name matching: %s -> %s", block.Activity, foundID)
			}
		}

		// If still not found, try to find the best matching POI from available ones
		if !found && len(poiMap) > 0 {
			bestMatch := p.findBestMatchingPOI(block.Activity, block.WhatToDo, poiMap)
			if bestMatch.ID != "" {
				mainPOI = bestMatch
				found = true
				log.Printf("Found POI by best match: %s -> %s", block.Activity, bestMatch.Name)
			}
		}

		if !found {
			log.Printf("Warning: No POI found for activity: %s, creating generic activity", block.Activity)
			// Create a generic activity without POI
			finalPlan = append(finalPlan, response_models.ActivityPlanBlock{
				Activity:  block.Activity,
				StartTime: block.StartTime,
				EndTime:   block.EndTime,
				MainPOI: response_models.ActivityPOI{
					ID:          "",
					Name:        extractLocationFromActivity(block.Activity),
					Description: block.WhatToDo,
				},
				Alternatives: nil,
				WhatToDo:     block.WhatToDo,
			})
			continue
		}

		// Find alternatives
		var alts []response_models.ActivityPOI
		for _, id := range block.AlternativePoiIDs {
			if alt, ok := poiMap[id]; ok {
				alts = append(alts, alt)
			}
		}

		// If no alternatives found, suggest some from the same category/area
		if len(alts) == 0 {
			alts = p.findSuggestedAlternatives(mainPOI, poiMap, 2)
		}

		finalPlan = append(finalPlan, response_models.ActivityPlanBlock{
			Activity:     block.Activity,
			StartTime:    block.StartTime,
			EndTime:      block.EndTime,
			MainPOI:      mainPOI,
			Alternatives: alts,
			WhatToDo:     block.WhatToDo,
		})
	}

	return finalPlan
}

// Find POI by matching names in activity description
func (p *PromptService) findPOIByNameInActivity(activity, whatToDo string, poiNameToID map[string]string) string {
	searchText := strings.ToLower(activity + " " + whatToDo)

	for poiName, poiID := range poiNameToID {
		// Check if POI name is mentioned in the activity
		if strings.Contains(searchText, poiName) {
			return poiID
		}

		// Also check for partial matches (for compound names)
		parts := strings.Fields(poiName)
		if len(parts) > 1 {
			for _, part := range parts {
				if len(part) > 3 && strings.Contains(searchText, part) {
					return poiID
				}
			}
		}
	}

	return ""
}

// Find best matching POI based on activity description
func (p *PromptService) findBestMatchingPOI(activity, whatToDo string, poiMap map[string]response_models.ActivityPOI) response_models.ActivityPOI {
	searchText := strings.ToLower(activity + " " + whatToDo)

	var bestMatch response_models.ActivityPOI
	maxScore := 0

	for _, poi := range poiMap {
		score := p.calculatePOIMatchScore(searchText, poi)
		if score > maxScore {
			maxScore = score
			bestMatch = poi
		}
	}

	return bestMatch
}

// Calculate match score between activity and POI
func (p *PromptService) calculatePOIMatchScore(searchText string, poi response_models.ActivityPOI) int {
	score := 0
	poiText := strings.ToLower(poi.Name + " " + poi.Description)

	// Check for direct name matches
	if strings.Contains(searchText, strings.ToLower(poi.Name)) {
		score += 10
	}

	// Check for tag matches
	for _, tag := range poi.Tags {
		tagLower := strings.ToLower(tag)
		if strings.Contains(searchText, tagLower) {
			score += 5
		}
	}

	// Check for description keyword matches
	keywords := []string{"temple", "pagoda", "lake", "fall", "market", "palace", "garden", "museum"}
	for _, keyword := range keywords {
		if strings.Contains(searchText, keyword) && strings.Contains(poiText, keyword) {
			score += 3
		}
	}

	return score
}

// Find suggested alternatives from the same category or nearby POIs
func (p *PromptService) findSuggestedAlternatives(mainPOI response_models.ActivityPOI, poiMap map[string]response_models.ActivityPOI, maxAlts int) []response_models.ActivityPOI {
	var alternatives []response_models.ActivityPOI

	for _, poi := range poiMap {
		if poi.ID == mainPOI.ID {
			continue // Skip the main POI itself
		}

		// Prefer POIs from the same category
		if poi.CategoryID == mainPOI.CategoryID {
			alternatives = append(alternatives, poi)
			if len(alternatives) >= maxAlts {
				break
			}
		}
	}

	// If not enough alternatives from same category, add others
	if len(alternatives) < maxAlts {
		for _, poi := range poiMap {
			if poi.ID == mainPOI.ID {
				continue
			}

			// Skip if already added
			alreadyAdded := false
			for _, alt := range alternatives {
				if alt.ID == poi.ID {
					alreadyAdded = true
					break
				}
			}

			if !alreadyAdded {
				alternatives = append(alternatives, poi)
				if len(alternatives) >= maxAlts {
					break
				}
			}
		}
	}

	return alternatives
}

// Extract location name from activity description as fallback
func extractLocationFromActivity(activity string) string {
	// Try to extract location names from activity
	words := strings.Fields(activity)
	for i, word := range words {
		// Look for capitalized words that might be place names
		if len(word) > 2 && strings.Title(word) == word {
			if i+1 < len(words) && strings.Title(words[i+1]) == words[i+1] {
				return word + " " + words[i+1] // Return compound names like "Xuan Huong"
			}
			return word
		}
	}
	return "Unknown Location"
}

// Add this method to your AI service interface
func (p *PromptService) generateStructuredPlanWithBetterFormat(ctx context.Context, userPrompt string, poiTextList []string, dayCount int) (string, error) {
	// Create a more structured prompt for the AI
	instruction := fmt.Sprintf(`
You are creating a %d-day travel itinerary. Here are the available POIs:

%s

IMPORTANT: 
1. Use the exact POI_ID from the list above for main_poi_id and alternative_poi_ids
2. Each activity MUST have a valid main_poi_id from the provided list
3. Provide 1-2 alternative POI IDs for each main activity
4. Times should be realistic (e.g., 09:00, 14:30)

User request: %s

Return JSON in this exact format:
`, dayCount, strings.Join(poiTextList, "\n"), userPrompt)

	if dayCount == 1 {
		instruction += `
[
  {
    "activity": "Visit [POI Name]",
    "start_time": "09:00",
    "end_time": "11:00", 
    "main_poi_id": "exact-poi-id-from-list",
    "alternative_poi_ids": ["alternative-poi-id-1", "alternative-poi-id-2"],
    "what_to_do": "Detailed description of what to do"
  }
]`
	} else {
		instruction += `
{
  "days": [
    {
      "day": 1,
      "date": "2024-01-01",
      "activities": [
        {
          "activity": "Visit [POI Name]",
          "start_time": "09:00",
          "end_time": "11:00",
          "main_poi_id": "exact-poi-id-from-list", 
          "alternative_poi_ids": ["alternative-poi-id-1"],
          "what_to_do": "Detailed description"
        }
      ]
    }
  ]
}`
	}

	return p.aiService.GenerateStructuredPlan(ctx, instruction, poiTextList, dayCount)
}

// Improved method to validate and fix POI IDs in the response
func (p *PromptService) validateAndFixPOIIDs(rawJSON string, poiMap map[string]response_models.ActivityPOI) string {
	// If the AI returns POI names instead of IDs, try to fix them
	for poiID, poi := range poiMap {
		// Replace POI names with their IDs in the JSON
		rawJSON = strings.ReplaceAll(rawJSON, fmt.Sprintf(`"%s"`, poi.Name), fmt.Sprintf(`"%s"`, poiID))
		rawJSON = strings.ReplaceAll(rawJSON, fmt.Sprintf(`"%s"`, strings.ToLower(poi.Name)), fmt.Sprintf(`"%s"`, poiID))
	}

	return rawJSON
}

// Multi-strategy POI finding
func (p *PromptService) findRelevantPOIs(ctx context.Context, userPrompt string) ([]*db_models.POI, error) {
	var allPOIs []*db_models.POI

	// Strategy 1: Location-based search
	locations := p.ExtractLocationFromPrompt(userPrompt)
	if len(locations) > 0 {
		log.Printf("Found locations in prompt: %v", locations)
		locationPOIs, err := p.findPOIsByLocation(ctx, locations)
		if err == nil && len(locationPOIs) > 0 {
			allPOIs = append(allPOIs, locationPOIs...)
			log.Printf("Found %d POIs by location search", len(locationPOIs))
		}
	}

	// Strategy 2: Embedding-based search (your existing logic)
	embeddingPOIs, err := p.findPOIsByEmbedding(ctx, userPrompt)
	if err == nil && len(embeddingPOIs) > 0 {
		allPOIs = p.mergePOIsWithoutDuplicates(allPOIs, embeddingPOIs)
		log.Printf("Total POIs after embedding search: %d", len(allPOIs))
	}

	// Strategy 3: Keyword-based fallback
	if len(allPOIs) < 5 {
		keywordPOIs, err := p.findPOIsByKeywords(ctx, userPrompt)
		if err == nil && len(keywordPOIs) > 0 {
			allPOIs = p.mergePOIsWithoutDuplicates(allPOIs, keywordPOIs)
			log.Printf("Total POIs after keyword search: %d", len(allPOIs))
		}
	}

	// Limit results to avoid overwhelming the AI
	if len(allPOIs) > 20 {
		allPOIs = allPOIs[:20]
	}

	return allPOIs, nil
}

// Find POIs by location names - you'll need to implement this in your repository
func (p *PromptService) findPOIsByLocation(ctx context.Context, locations []string) ([]*db_models.POI, error) {

	var allPOIs []*db_models.POI

	// You can implement a more sophisticated location search here
	// For now, we'll search by POI names containing the location
	pois, err := p.poisRepo.FindPOIsByLocationNames(ctx, locations)
	if err == nil {
		allPOIs = append(allPOIs, pois...)
	}

	return allPOIs, nil
}

// Find POIs using embedding (your existing logic)
func (p *PromptService) findPOIsByEmbedding(ctx context.Context, userPrompt string) ([]*db_models.POI, error) {
	embedding, err := p.aiService.GetEmbedding(ctx, userPrompt)
	if err != nil {
		return nil, err
	}

	embeddedPois, err := p.embededRepo.GetListOfPoiEmbededByVector(embedding, nil)
	if err != nil || len(embeddedPois) == 0 {
		return nil, fmt.Errorf("no POIs found via embedding")
	}

	var poiIDs []string
	for _, ep := range embeddedPois {
		poiIDs = append(poiIDs, ep.PoiID)
	}

	result, err := p.poisRepo.ListPoisByPoisId(ctx, poiIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve POIs by IDs: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no POIs found for the provided embedding")
	}
	return result, nil
}

// Find POIs by keywords (fallback method)
func (p *PromptService) findPOIsByKeywords(ctx context.Context, userPrompt string) ([]*db_models.POI, error) {
	keywords := p.extractKeywords(userPrompt)
	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords found")
	}

	// You'll need to implement this in your repository
	return p.poisRepo.SearchPOIsByKeywords(ctx, keywords)
}

// Extract keywords from prompt
func (p *PromptService) extractKeywords(prompt string) []string {
	lower := strings.ToLower(prompt)

	travelKeywords := []string{
		"restaurant", "food", "eat", "dining", "cafe", "coffee",
		"hotel", "accommodation", "stay", "lodge", "resort",
		"temple", "pagoda", "church", "museum", "gallery",
		"park", "garden", "nature", "mountain", "beach", "lake",
		"shopping", "market", "mall", "souvenir",
		"nightlife", "bar", "club", "entertainment",
		"adventure", "hiking", "trekking", "cycling",
		"culture", "history", "traditional", "local",

		// Vietnamese keywords
		"nhà hàng", "ăn", "quán", "cà phê",
		"khách sạn", "nghỉ", "resort",
		"chùa", "đền", "bảo tàng",
		"công viên", "núi", "biển", "hồ",
		"chợ", "mua sắm",
		"văn hóa", "lịch sử", "truyền thống",
	}

	var foundKeywords []string
	for _, keyword := range travelKeywords {
		if strings.Contains(lower, keyword) {
			foundKeywords = append(foundKeywords, keyword)
		}
	}

	return foundKeywords
}

// Merge POI lists without duplicates
func (p *PromptService) mergePOIsWithoutDuplicates(existing, new []*db_models.POI) []*db_models.POI {
	seen := make(map[string]bool)
	var result []*db_models.POI

	// Add existing POIs
	for _, poi := range existing {
		if !seen[poi.ID.String()] {
			seen[poi.ID.String()] = true
			result = append(result, poi)
		}
	}

	// Add new POIs
	for _, poi := range new {
		if !seen[poi.ID.String()] {
			seen[poi.ID.String()] = true
			result = append(result, poi)
		}
	}

	return result
}

// Parse single day plan
func (p *PromptService) parseSingleDayPlan(rawJSON string, poiMap map[string]response_models.ActivityPOI) ([]response_models.ActivityPlanBlock, error) {
	var skeleton []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}

	if err := json.Unmarshal([]byte(rawJSON), &skeleton); err != nil {
		return nil, fmt.Errorf("invalid single-day plan JSON: %w", err)
	}

	return p.buildActivityBlocks(skeleton, poiMap), nil
}

// Parse multi-day plan
func (p *PromptService) parseMultiDayPlan(rawJSON string, poiMap map[string]response_models.ActivityPOI) ([]response_models.ActivityPlanBlock, error) {
	var multiDayPlan struct {
		Days []struct {
			Day        int    `json:"day"`
			Date       string `json:"date,omitempty"`
			Activities []struct {
				Activity          string   `json:"activity"`
				StartTime         string   `json:"start_time"`
				EndTime           string   `json:"end_time"`
				MainPoiID         string   `json:"main_poi_id"`
				AlternativePoiIDs []string `json:"alternative_poi_ids"`
				WhatToDo          string   `json:"what_to_do"`
			} `json:"activities"`
		} `json:"days"`
	}

	if err := json.Unmarshal([]byte(rawJSON), &multiDayPlan); err != nil {
		return nil, fmt.Errorf("invalid multi-day plan JSON: %w", err)
	}

	var allActivities []response_models.ActivityPlanBlock

	for _, day := range multiDayPlan.Days {
		// Add day separator
		dayHeader := response_models.ActivityPlanBlock{
			Activity:  fmt.Sprintf("Day %d", day.Day),
			StartTime: "00:00",
			EndTime:   "23:59",
			WhatToDo:  fmt.Sprintf("Activities for day %d", day.Day),
		}
		if day.Date != "" {
			dayHeader.WhatToDo += fmt.Sprintf(" (%s)", day.Date)
		}
		allActivities = append(allActivities, dayHeader)

		// Add activities for this day
		dayActivities := p.buildActivityBlocks(day.Activities, poiMap)
		allActivities = append(allActivities, dayActivities...)
	}

	return allActivities, nil
}

// Build activity blocks
func (p *PromptService) buildActivityBlocks(activities interface{}, poiMap map[string]response_models.ActivityPOI) []response_models.ActivityPlanBlock {
	var finalPlan []response_models.ActivityPlanBlock

	// Convert activities to the expected format
	var activityList []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}

	// Handle different activity types
	switch v := activities.(type) {
	case []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}:
		activityList = v
	default:
		// Convert via JSON marshaling/unmarshaling
		jsonData, _ := json.Marshal(activities)
		json.Unmarshal(jsonData, &activityList)
	}

	for _, block := range activityList {
		mainPOI, ok := poiMap[block.MainPoiID]
		if !ok {
			log.Printf("Warning: POI %s not found, skipping activity: %s", block.MainPoiID, block.Activity)
			continue
		}

		var alts []response_models.ActivityPOI
		for _, id := range block.AlternativePoiIDs {
			if alt, ok := poiMap[id]; ok {
				alts = append(alts, alt)
			}
		}

		finalPlan = append(finalPlan, response_models.ActivityPlanBlock{
			Activity:     block.Activity,
			StartTime:    block.StartTime,
			EndTime:      block.EndTime,
			MainPOI:      mainPOI,
			Alternatives: alts,
			WhatToDo:     block.WhatToDo,
		})
	}

	return finalPlan
}

func flattenTags(tags []*db_models.Tag) []string {
	var out []string
	for _, tag := range tags {
		if tag == nil {
			continue
		}
		// Combine both language names (e.g., "waterfall/thác nước")
		if tag.EnName != "" && tag.ViName != "" {
			out = append(out, fmt.Sprintf("%s/%s", tag.EnName, tag.ViName))
		} else if tag.EnName != "" {
			out = append(out, tag.EnName)
		} else if tag.ViName != "" {
			out = append(out, tag.ViName)
		}
	}
	return out
}
