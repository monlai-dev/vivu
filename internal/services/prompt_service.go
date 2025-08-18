package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type PromptServiceInterface interface {
	CreatePrompt(ctx context.Context, prompt string) (string, error)
	PromptInput(ctx context.Context, request request_models.CreateTagRequest) (string, error)
	CreateAIPlan(ctx context.Context, userPrompt string) ([]response_models.ActivityPlanBlock, error)
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
		return "", fmt.Errorf("failed to get embedding: %w", err)
	}

	// Get similar POIs based on vector similarity
	poiEmbeddedIds, err := p.embededRepo.GetListOfPoiEmbededByVector(vector, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get embedded POIs: %w", err)
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
		return "", fmt.Errorf("failed to get POI details: %w", err)
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
		return "", fmt.Errorf("tag name is required")
	}

	searchPrompt := fmt.Sprintf("Find places related to %s", request.En)
	if request.Vi != "" {
		searchPrompt += fmt.Sprintf(" (%s)", request.Vi)
	}

	return p.CreatePrompt(ctx, searchPrompt)
}

func (p *PromptService) CreateAIPlan(ctx context.Context, userPrompt string) ([]response_models.ActivityPlanBlock, error) {
	// Validate input
	if strings.TrimSpace(userPrompt) == "" {
		return nil, fmt.Errorf("user prompt cannot be empty")
	}

	// Find POIs using multiple strategies
	pois, err := p.findRelevantPOIs(ctx, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to find relevant POIs: %w", err)
	}

	if len(pois) == 0 {
		return nil, fmt.Errorf("no relevant POIs found for your request. Please try being more specific about activities or locations")
	}

	// Prepare POI data for AI
	var poiTextList []string
	poiMap := make(map[string]response_models.ActivityPOI)

	for _, poi := range pois {
		poiText := fmt.Sprintf("%s (ID: %s): %s", poi.Name, poi.ID, poi.Description)
		if poi.Address != "" {
			poiText += fmt.Sprintf(" - Located at: %s", poi.Address)
		}
		if poi.OpeningHours != "" {
			poiText += fmt.Sprintf(" - Hours: %s", poi.OpeningHours)
		}

		poiTextList = append(poiTextList, poiText)
		poiMap[poi.ID.String()] = response_models.ActivityPOI{
			ID:          poi.ID.String(),
			Name:        poi.Name,
			Description: poi.Description,
			ProvinceID:  poi.ProvinceID.String(),
			CategoryID:  poi.CategoryID.String(),
			Tags:        flattenTags(poi.Tags),
		}
	}

	dayCount := extractDayCount(userPrompt)
	log.Printf("Planning for %d day(s) with %d POIs", dayCount, len(pois))

	// Generate AI plan
	rawJSON, err := p.aiService.GenerateStructuredPlan(ctx, userPrompt, poiTextList, dayCount)
	if err != nil {
		return nil, fmt.Errorf("failed to generate AI plan: %w", err)
	}

	// Parse response based on day count
	if dayCount > 1 {
		return p.parseMultiDayPlan(rawJSON, poiMap)
	}

	return p.parseSingleDayPlan(rawJSON, poiMap)
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
	// This requires adding a method to your POI repository
	// For now, we'll use a simple name-based search
	var allPOIs []*db_models.POI

	for _, location := range locations {
		// You can implement a more sophisticated location search here
		// For now, we'll search by POI names containing the location
		pois, err := p.poisRepo.SearchPOIsByName(ctx, location)
		if err == nil {
			allPOIs = append(allPOIs, pois...)
		}
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
			log.Printf("Warning: Main POI %s not found, skipping activity: %s", block.MainPoiID, block.Activity)
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

func extractDayCount(prompt string) int {
	lower := strings.ToLower(prompt)

	// Check for explicit day mentions
	for i := 1; i <= 14; i++ {
		patterns := []string{
			fmt.Sprintf("%d days", i),
			fmt.Sprintf("%d ngày", i),
			fmt.Sprintf("%d day", i),
			fmt.Sprintf("%d-day", i),
		}

		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				return i
			}
		}
	}

	// Check for written numbers
	writtenNumbers := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"một": 1, "hai": 2, "ba": 3, "bốn": 4, "năm": 5,
		"sáu": 6, "bảy": 7, "tám": 8, "chín": 9, "mười": 10,
	}

	for word, num := range writtenNumbers {
		if strings.Contains(lower, word+" day") || strings.Contains(lower, word+" ngày") {
			return num
		}
	}

	// Check for weekend/week patterns
	if strings.Contains(lower, "weekend") || strings.Contains(lower, "cuối tuần") {
		return 2
	}
	if strings.Contains(lower, "week") || strings.Contains(lower, "tuần") {
		return 7
	}

	return 1 // default to 1-day
}
