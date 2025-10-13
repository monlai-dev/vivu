package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	CreateNarrativeAIPlan(ctx context.Context, userPrompt string) (*response_models.TravelItinerary, error)
	ExtractLocationFromPrompt(prompt string) []string

	StartTravelQuiz(ctx context.Context, userID string) (*response_models.QuizResponse, error)
	ProcessQuizAnswer(ctx context.Context, request request_models.QuizRequest) (*response_models.QuizResponse, error)
	GeneratePersonalizedPlan(ctx context.Context, sessionID string) (*response_models.QuizResultResponse, error)

	GeneratePlanOnly(ctx context.Context, sessionID, userId string) (*response_models.PlanOnly, error)
	GeneratePlanAndSave(ctx context.Context, sessionID string, userId uuid.UUID) (uuid.UUID, error)
}

var vnLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*60*60)
	}
	return loc
}()

type planModelProfile struct {
	Destination  string   `json:"destination"`
	DurationDays int      `json:"duration_days"`
	BudgetRange  string   `json:"budget_range,omitempty"`
	PartySize    int      `json:"party_size,omitempty"`
	StartDate    string   `json:"start_date,omitempty"` // "YYYY-MM-DD" (VN)
	EndDate      string   `json:"end_date,omitempty"`   // "YYYY-MM-DD" (VN)
	TravelStyle  []string `json:"travel_style,omitempty"`
	Interests    []string `json:"interests,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type PromptService struct {
	poisService    POIServiceInterface
	tagService     TagServiceInterface
	aiService      utils.EmbeddingClientInterface
	embededRepo    repositories.IPoiEmbededRepository
	poisRepo       repositories.POIRepository
	quizSessions   map[string]*QuizSession
	sessionMutex   sync.RWMutex
	matrixSvc      DistanceMatrixService
	journeyRepo    repositories.JourneyRepository
	accountSerivce AccountServiceInterface
}

func NewPromptService(
	poisService POIServiceInterface,
	tagService TagServiceInterface,
	aiService utils.EmbeddingClientInterface,
	embededRepo repositories.IPoiEmbededRepository,
	poisRepo repositories.POIRepository,
	matrixSvc DistanceMatrixService,
	journeyRepo repositories.JourneyRepository,
	accountService AccountServiceInterface,
) PromptServiceInterface {
	return &PromptService{
		poisService:    poisService,
		tagService:     tagService,
		aiService:      aiService,
		embededRepo:    embededRepo,
		poisRepo:       poisRepo,
		matrixSvc:      matrixSvc,
		journeyRepo:    journeyRepo,
		accountSerivce: accountService,
	}
}

type QuizSession struct {
	SessionID   string            `json:"session_id"`
	UserID      string            `json:"user_id"`
	Answers     map[string]string `json:"answers"`
	CurrentStep int               `json:"current_step"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ---------- Plan generate & save ----------

func (p *PromptService) GeneratePlanAndSave(ctx context.Context, sessionID string, userId uuid.UUID) (uuid.UUID, error) {
	plan, err := p.GeneratePlanOnly(ctx, sessionID, userId.String())
	if err != nil {
		return uuid.Nil, err
	}
	resultUUid := p.savePlanAsyncWithRetry(sessionID, userId, plan)
	if resultUUid == uuid.Nil {
		return uuid.Nil, fmt.Errorf("failed to save plan after retries")
	}

	return resultUUid, nil
}

func (p *PromptService) savePlanAsyncWithRetry(sessionID string, userId uuid.UUID, plan *response_models.PlanOnly) uuid.UUID {
	const (
		maxAttempts     = 5
		baseDelay       = 300 * time.Millisecond
		totalTimeBudget = 2 * time.Minute
	)

	ctx, cancel := context.WithTimeout(context.Background(), totalTimeBudget)
	defer cancel()

	var result uuid.UUID
	var err error

	jitter := func(d time.Duration) time.Duration {
		n := rand.New(rand.NewSource(time.Now().UnixNano()))
		variance := time.Duration(n.Int63n(int64(d))) - d/2
		return d + variance
	}

	// Pull start date from the quiz session (VN tz); fallback to VN today
	p.sessionMutex.RLock()
	sess := p.quizSessions[sessionID]
	p.sessionMutex.RUnlock()

	startVN := time.Now().In(vnLoc)
	if sess != nil {
		if sd, ok := sess.Answers["start_date"]; ok {
			if dt, err := parseDateVN(sd); err == nil {
				startVN = dt
			}
		}
	}
	// normalize to midnight VN
	startVN = time.Date(startVN.Year(), startVN.Month(), startVN.Day(), 0, 0, 0, 0, vnLoc)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err = p.journeyRepo.ReplaceMaterializedPlan(ctx, &uuid.Nil, plan, &repositories.CreateJourneyInput{
			Title:     fmt.Sprintf("Trip to %s", plan.Destination),
			AccountID: userId,
			StartDate: startVN,
		})
		if err == nil {
			log.Printf("[plan] saved (session=%s, attempt=%d)", sessionID, attempt)
			return result
		}

		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			log.Printf("[plan] aborting retries due to context end (session=%s, attempt=%d, err=%v)", sessionID, attempt, err)
			return uuid.Nil
		}

		delay := time.Duration(1<<uint(attempt-1)) * baseDelay
		sleep := jitter(delay)
		log.Printf("[plan] save failed; retrying in %v (session=%s, attempt=%d/%d, err=%v)", sleep, sessionID, attempt, maxAttempts, err)
		time.Sleep(sleep)
	}

	log.Printf("[plan] giving up after %d attempts (session=%s)", maxAttempts, sessionID)

	return uuid.Nil
}

func (p *PromptService) GeneratePlanOnly(ctx context.Context, sessionID, userId string) (*response_models.PlanOnly, error) {
	p.sessionMutex.RLock()
	session, ok := p.quizSessions[sessionID]
	p.sessionMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("quiz session not found")
	}

	startTime := time.Now()
	log.Printf("Generating plan only for session %s", sessionID)

	profile := p.createTravelProfile(session.Answers) // computes Duration from start/end

	if profile.Duration < 1 {
		profile.Duration = 1
	}

	userHaveSubcriptions, err := p.accountSerivce.IsUserHaveSubscription(userId)
	if err != nil {

		return nil, fmt.Errorf("failed to check user subscription: %w", err)
	}

	fmt.Printf("userwithid %s have sub: %v", userId, userHaveSubcriptions)

	if profile.Duration > 3 && userHaveSubcriptions == false {
		return nil, fmt.Errorf("free users can only create up to 3-day itineraries. Please subscribe for longer trips")
	}

	pois, err := p.findPersonalizedPOIs(ctx, profile)
	if err != nil || len(pois) == 0 {
		return nil, fmt.Errorf("no relevant POIs")
	}

	var list []request_models.POISummary
	for _, poi := range pois {
		list = append(list, request_models.POISummary{
			ID: poi.ID.String(), Name: poi.Name, Category: p.categorizePOI(poi), Description: poi.Description,
		})
		if len(list) >= 20 {
			break
		}
	}

	dayCount := profile.Duration

	var startStr, endStr string
	if sd := strings.TrimSpace(session.Answers["start_date"]); sd != "" {
		if dt, err := parseDateVN(sd); err == nil {
			startStr = dt.Format("2006-01-02")
		}
	}
	if ed := strings.TrimSpace(session.Answers["end_date"]); ed != "" {
		if dt, err := parseDateVN(ed); err == nil {
			endStr = dt.Format("2006-01-02")
		}
	}

	party := 0
	if paxStr := strings.TrimSpace(session.Answers["num_customers"]); paxStr != "" {
		if pax, err := strconv.Atoi(paxStr); err == nil && pax > 0 {
			party = pax
		}
	}

	// Explicit tags from session (comma-separated). If you already put some in TravelStyle,
	// that’s fine; we still pass them separately as `Tags` so the model can key on that signal.
	var tags []string
	if rawTags, ok := session.Answers["tags"]; ok {
		tags = parseCSVTags(rawTags)
	}

	payload := planModelProfile{
		Destination:  profile.Destination,
		DurationDays: dayCount,
		BudgetRange:  profile.BudgetRange,
		PartySize:    party,
		StartDate:    startStr,
		EndDate:      endStr,
		TravelStyle:  append([]string{}, profile.TravelStyle...), // copy
		Interests:    append([]string{}, profile.Interests...),   // copy
		Tags:         tags,
	}

	jsonPlan, err := p.aiService.GeneratePlanOnlyJSON(ctx, payload, list, dayCount)
	if err != nil {
		return nil, err
	}

	var plan response_models.PlanOnly
	if err := json.Unmarshal([]byte(jsonPlan), &plan); err != nil {
		return nil, fmt.Errorf("invalid plan json: %w", err)
	}

	if len(plan.Days) != dayCount {
		return nil, fmt.Errorf("expected %d days, got %d", dayCount, len(plan.Days))
	}

	uniq := make(map[string]struct{})
	for _, d := range plan.Days {
		for _, act := range d.Activities {
			if act.MainPOIID != "" {
				uniq[act.MainPOIID] = struct{}{}
			}
		}
	}
	if len(uniq) == 0 {
		return nil, fmt.Errorf("plan contains no poi ids")
	}

	ids := make([]string, 0, len(uniq))
	for id := range uniq {
		ids = append(ids, id)
	}

	dbPOIs, err := p.poisRepo.ListPoisByPoisId(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load pois for enrichment: %w", err)
	}

	respByID := make(map[string]response_models.POI, len(dbPOIs))
	for _, poi := range dbPOIs {
		respByID[poi.ID.String()] = response_models.POI{
			ID:           poi.ID.String(),
			Name:         poi.Name,
			Latitude:     poi.Latitude,
			Longitude:    poi.Longitude,
			Category:     poi.Category.Name,
			OpeningHours: poi.OpeningHours,
			ContactInfo:  poi.ContactInfo,
			Address:      poi.Address,
			PoiDetails: func() *response_models.PoiDetails {
				if poi.Details.ID == uuid.Nil {
					return nil
				}
				return &response_models.PoiDetails{
					ID:          poi.Details.ID.String(),
					Description: poi.Description,
					Image:       poi.Details.Images,
				}
			}(),
		}
	}

	for di := range plan.Days {
		for ai := range plan.Days[di].Activities {
			poid := plan.Days[di].Activities[ai].MainPOIID
			if poid == "" {
				continue
			}
			if pinfo, ok := respByID[poid]; ok {
				poiCopy := pinfo
				plan.Days[di].Activities[ai].MainPOI = &poiCopy
			}
		}
	}

	// Build distance matrix + legs as before
	idList := make([]string, 0, len(respByID))
	for id := range respByID {
		idList = append(idList, id)
	}
	points := make([]MatrixPoint, 0, len(idList))
	for _, id := range idList {
		poi := respByID[id]
		points = append(points, MatrixPoint{ID: id, Lat: poi.Latitude, Lng: poi.Longitude})
	}
	distMat, err := p.matrixSvc.ComputeDistances(ctx, points)
	if err == nil {
		plan.DistanceMatrix = make(response_models.DistanceMatrix, len(distMat))
		for fromID, row := range distMat {
			if _, ok := plan.DistanceMatrix[fromID]; !ok {
				plan.DistanceMatrix[fromID] = map[string]response_models.MatrixEdge{}
			}
			for toID, edge := range row {
				plan.DistanceMatrix[fromID][toID] = response_models.MatrixEdge{DistanceMeters: edge.DistanceMeters}
			}
		}
	}

	for di := range plan.Days {
		acts := plan.Days[di].Activities
		for ai := 0; ai+1 < len(acts); ai++ {
			from := plan.Days[di].Activities[ai].MainPOI
			to := plan.Days[di].Activities[ai+1].MainPOI
			if from == nil || to == nil {
				continue
			}
			var dPtr *int
			if plan.DistanceMatrix != nil {
				if row, ok := plan.DistanceMatrix[from.ID]; ok {
					if cell, ok := row[to.ID]; ok {
						d := cell.DistanceMeters
						dPtr = &d
						plan.Days[di].Activities[ai].DistanceToNextMeters = dPtr
					}
				}
			}
			url := BuildGoogleDirURL(from.Latitude, from.Longitude, to.Latitude, to.Longitude)
			plan.Days[di].Activities[ai].NextLegMapURL = url
			from.DistanceToNextMeters = dPtr
			from.NextLegMapURL = url
		}
	}

	plan.CreatedAt = time.Now()
	log.Printf("Enriched plan with distances and URLs in %.3f ms", time.Since(startTime).Seconds())
	return &plan, nil
}

// ---------- Utils ----------

// parseCSVTags splits by comma, trims, and drops empties.
func parseCSVTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func BuildGoogleDirURL(originLat, originLng, destLat, destLng float64) string {
	q := url.Values{}
	q.Set("api", "1")
	q.Set("origin", fmt.Sprintf("%f,%f", originLat, originLng))
	q.Set("destination", fmt.Sprintf("%f,%f", destLat, destLng))
	q.Set("travelmode", "driving")
	return "https://www.google.com/maps/dir/?" + q.Encode()
}

// ---------- Quiz flow (reworked) ----------

func (p *PromptService) StartTravelQuiz(ctx context.Context, userID string) (*response_models.QuizResponse, error) {
	sessionID := fmt.Sprintf("quiz_%s_%d", userID, time.Now().Unix())

	session := &QuizSession{
		SessionID:   sessionID,
		UserID:      userID,
		Answers:     make(map[string]string),
		CurrentStep: 1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	p.sessionMutex.Lock()
	if p.quizSessions == nil {
		p.quizSessions = make(map[string]*QuizSession)
	}
	p.quizSessions[sessionID] = session
	p.sessionMutex.Unlock()

	questions := p.generateQuizQuestions()

	return &response_models.QuizResponse{
		Questions:    []request_models.QuizQuestion{questions[0]},
		CurrentStep:  1,
		TotalSteps:   len(questions),
		SessionID:    sessionID,
		IsComplete:   false,
		NextEndpoint: "/api/quiz/answer",
	}, nil
}

func (p *PromptService) ProcessQuizAnswer(ctx context.Context, request request_models.QuizRequest) (*response_models.QuizResponse, error) {
	p.sessionMutex.Lock()
	session, exists := p.quizSessions[request.SessionID]
	if !exists {
		p.sessionMutex.Unlock()
		return nil, fmt.Errorf("quiz session not found")
	}
	for key, value := range request.Answers {
		session.Answers[key] = strings.TrimSpace(value)
	}
	session.UpdatedAt = time.Now()
	p.sessionMutex.Unlock()

	questions := p.generateQuizQuestions()

	// validate step input where helpful (dates/pax)
	switch session.CurrentStep {
	case 2: // start_date
		if sd := session.Answers["start_date"]; sd != "" {
			if _, err := parseDateVN(sd); err != nil {
				return &response_models.QuizResponse{
					Questions: []request_models.QuizQuestion{{
						ID:       "start_date",
						Question: "Please enter a valid start date (YYYY-MM-DD, VN time) 📅",
						Type:     "text",
						Required: true,
						Category: "dates",
					}},
					CurrentStep:  session.CurrentStep,
					TotalSteps:   len(questions),
					SessionID:    request.SessionID,
					IsComplete:   false,
					NextEndpoint: "/api/quiz/answer",
				}, nil
			}
		}
	case 3: // end_date
		if ed := session.Answers["end_date"]; ed != "" {
			if _, err := parseDateVN(ed); err != nil {
				return &response_models.QuizResponse{
					Questions: []request_models.QuizQuestion{{
						ID:       "end_date",
						Question: "Please enter a valid end date (YYYY-MM-DD, VN time) 📅",
						Type:     "text",
						Required: true,
						Category: "dates",
					}},
					CurrentStep:  session.CurrentStep,
					TotalSteps:   len(questions),
					SessionID:    request.SessionID,
					IsComplete:   false,
					NextEndpoint: "/api/quiz/answer",
				}, nil
			}
		}
	}

	if session.CurrentStep >= len(questions) {
		return &response_models.QuizResponse{
			Questions:    nil,
			CurrentStep:  session.CurrentStep,
			TotalSteps:   len(questions),
			SessionID:    request.SessionID,
			IsComplete:   true,
			NextEndpoint: "/api/quiz/generate-plan",
		}, nil
	}

	session.CurrentStep++
	nextQuestion := questions[session.CurrentStep-1]

	return &response_models.QuizResponse{
		Questions:    []request_models.QuizQuestion{nextQuestion},
		CurrentStep:  session.CurrentStep,
		TotalSteps:   len(questions),
		SessionID:    request.SessionID,
		IsComplete:   false,
		NextEndpoint: "/api/quiz/answer",
	}, nil
}

// Only collect: destination, start_date, end_date, num_customers, budget
func (p *PromptService) generateQuizQuestions() []request_models.QuizQuestion {
	return []request_models.QuizQuestion{
		{
			ID:       "destination",
			Question: "Where are you traveling to? 🌍 (e.g., Da Lat, Ho Chi Minh City)",
			Type:     "text", // keep text to allow free input / locales
			Required: true,
			Category: "destination",
		},
		{
			ID:       "start_date",
			Question: "When does your trip start? 📅 (YYYY-MM-DD, VN time)",
			Type:     "text",
			Required: true,
			Category: "dates",
		},
		{
			ID:       "end_date",
			Question: "When does your trip end? 📅 (YYYY-MM-DD, VN time)",
			Type:     "text",
			Required: true,
			Category: "dates",
		},
		{
			ID:       "num_customers",
			Question: "How many travelers are going? 👥",
			Type:     "single_choice",
			Options:  []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			Required: true,
			Category: "party",
		},
		{
			ID:       "budget",
			Question: "What is your budget per person per day? 💰",
			Type:     "single_choice",
			Options:  []string{"$0-30", "$31-70", "$71-150", "$151-300", "$300+"},
			Required: true,
			Category: "budget",
		},
	}
}

// ---------- Personalized plan (uses the new inputs) ----------

func (p *PromptService) GeneratePersonalizedPlan(ctx context.Context, sessionID string) (*response_models.QuizResultResponse, error) {
	p.sessionMutex.RLock()
	session, exists := p.quizSessions[sessionID]
	p.sessionMutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("quiz session not found")
	}

	profile := p.createTravelProfile(session.Answers) // Duration computed from dates
	personalizedPrompt := p.buildPersonalizedPrompt(session.Answers)

	relevantPOIs, err := p.findPersonalizedPOIs(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to find relevant POIs: %w", err)
	}

	itinerary, err := p.CreateNarrativeAIPlan(ctx, personalizedPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate itinerary: %w", err)
	}

	recommendations := p.generatePersonalizedRecommendations(relevantPOIs, profile, session.Answers)

	return &response_models.QuizResultResponse{
		SessionID:       sessionID,
		UserProfile:     profile,
		Itinerary:       itinerary,
		Recommendations: recommendations,
	}, nil
}

// ---------- Profile & Prompt building (updated to dates/pax/budget) ----------

func (p *PromptService) createTravelProfile(answers map[string]string) response_models.TravelProfile {
	profile := response_models.TravelProfile{
		TravelStyle: []string{},
		Interests:   []string{},
	}

	// destination
	if dest, ok := answers["destination"]; ok {
		profile.Destination = p.parseDestination(dest)
	}

	// dates -> duration (inclusive of start day)
	var start, end *time.Time
	if sd, ok := answers["start_date"]; ok && strings.TrimSpace(sd) != "" {
		if dt, err := parseDateVN(sd); err == nil {
			start = &dt
		}
	}
	if ed, ok := answers["end_date"]; ok && strings.TrimSpace(ed) != "" {
		if dt, err := parseDateVN(ed); err == nil {
			end = &dt
		}
	}
	if start != nil && end != nil && !end.Before(*start) {
		days := int(end.Sub(*start).Hours()/24) + 1
		if days < 1 {
			days = 1
		}
		profile.Duration = days
	} else {
		profile.Duration = 1
	}

	// budget
	if budget, ok := answers["budget"]; ok {
		profile.BudgetRange = budget
	}

	// party size (store in Interests as meta tag if TravelProfile lacks a dedicated field)
	if paxStr, ok := answers["num_customers"]; ok {
		if pax, err := strconv.Atoi(strings.TrimSpace(paxStr)); err == nil && pax > 0 {
			// add a soft tag the models can read
			profile.Interests = append(profile.Interests, fmt.Sprintf("party:%d", pax))
		}
	}

	if tags, ok := answers["tags"]; ok && strings.TrimSpace(tags) != "" {
		tagList := strings.Split(tags, ",")
		for _, tag := range tagList {
			profile.TravelStyle = append(profile.TravelStyle, strings.TrimSpace(tag))
		}
	}

	// fallback minimums
	if profile.Destination == "" {
		profile.Destination = "Vietnam"
	}
	return profile
}

func (p *PromptService) buildPersonalizedPrompt(answers map[string]string) string {
	var b strings.Builder

	dest := "Vietnam"
	if v := strings.TrimSpace(answers["destination"]); v != "" {
		dest = v
	}

	start := strings.TrimSpace(answers["start_date"])
	end := strings.TrimSpace(answers["end_date"])
	pax := strings.TrimSpace(answers["num_customers"])
	if pax == "" {
		pax = "1"
	}
	budget := strings.TrimSpace(answers["budget"])
	if budget == "" {
		budget = "$31-70"
	}

	// compute duration for the LLM as well
	durationDays := 1
	if sd, err := parseDateVN(start); err == nil {
		if ed, err2 := parseDateVN(end); err2 == nil && !ed.Before(sd) {
			durationDays = int(ed.Sub(sd).Hours()/24) + 1
		}
	}

	b.WriteString("Create a personalized travel itinerary based on these inputs:\n\n")
	b.WriteString(fmt.Sprintf("Destination: %s\n", dest))
	if start != "" && end != "" {
		b.WriteString(fmt.Sprintf("Dates: %s to %s (VN time)\n", start, end))
	}
	b.WriteString(fmt.Sprintf("Duration: %d days\n", durationDays))
	b.WriteString(fmt.Sprintf("Travelers: %s people\n", pax))
	b.WriteString(fmt.Sprintf("Budget per person per day: %s\n", budget))
	b.WriteString("\nConstraints:\n- Use realistic times per activity\n- Cluster activities geographically when possible\n- Include food suggestions that match the budget\n- Prefer family-friendly options if party > 2 adults\n")
	b.WriteString("\nReturn a detailed, structured plan (JSON acceptable) with days and activities.\n")

	return b.String()
}

// ---------- Existing helpers (kept), lightly adapted ----------

func parseDateVN(s string) (time.Time, error) {
	// Expect YYYY-MM-DD
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), vnLoc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
	}
	// normalize to VN midnight
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, vnLoc), nil
}

func (p *PromptService) parseDestination(dest string) string {
	low := strings.ToLower(dest)
	switch {
	case strings.Contains(low, "da lat"):
		return "Da Lat, Vietnam"
	case strings.Contains(low, "ho chi minh"), strings.Contains(low, "saigon"):
		return "Ho Chi Minh City, Vietnam"
	case strings.Contains(low, "ha noi"), strings.Contains(low, "hanoi"):
		return "Hanoi, Vietnam"
	case strings.Contains(low, "hoi an"):
		return "Hoi An, Vietnam"
	case strings.Contains(low, "nha trang"):
		return "Nha Trang, Vietnam"
	case strings.Contains(low, "phu quoc"):
		return "Phu Quoc, Vietnam"
	default:
		return strings.TrimSpace(dest)
	}
}

// (Everything below here is your existing implementation, unchanged,
// except where it references profile.Duration (now computed from dates),
// or where prompts mention duration. I’ve left the rest intact.)

// ... [KEEP your CreatePrompt, PromptInput, narrative AI plan pipeline, POI conversion,
// categorizePOI, estimateDuration, estimatePriceLevel, generateTravelTags,
// generatePOITips, formatDestination, generateNarrativeAIPlan, buildNarrativePrompt,
// buildNarrativeItinerary, createFallbackNarrativeItinerary, cleanJSONResponse,
// generateSubtitle, inferTravelStyle, generateOverview, generateDayTheme,
// extractDayNumber, createStructuredPrompt, validateJSONStructure, cleanAndFixJSON,
// extractDayCount (still used by narrative generator for free-form prompts),
// callAIServiceWithStructuredPrompt, buildExplicitAIPrompt, tryConvertSingleToMultiDay,
// generateAIPlanWithRetry, buildUltraExplicitAIPrompt, convertSingleToMultiDayJSON,
// extractLocationFromActivity, generateStructuredPlanWithBetterFormat,
// findRelevantPOIs + strategies, findPOIsByLocation, findPOIsByEmbedding,
// findPOIsByKeywords, extractKeywords, mergePOIsWithoutDuplicates ] ...

// NOTE: No functional edits required to these blocks for the new quiz inputs.

func (p *PromptService) parseDuration(duration string) int {
	for i := 1; i <= 7; i++ {
		if strings.Contains(duration, fmt.Sprintf("%d day", i)) || strings.Contains(duration, fmt.Sprintf("%d days", i)) {
			return i
		}
	}

	return 1
}

func (p *PromptService) parseTravelStyle(style string) string {
	if strings.Contains(strings.ToLower(style), "adventure") {
		return "adventure"
	}
	if strings.Contains(strings.ToLower(style), "cultural") {
		return "cultural"
	}
	if strings.Contains(strings.ToLower(style), "romantic") {
		return "romantic"
	}
	// Add more style parsing...
	return "leisure"
}

func (p *PromptService) parseInterests(interests string) []string {
	// Parse comma-separated or multiple choice interests
	return strings.Split(interests, ",")
}

// findPersonalizedPOIs finds POIs that match the user's profile
func (p *PromptService) findPersonalizedPOIs(ctx context.Context, profile response_models.TravelProfile) ([]*db_models.POI, error) {
	// Combine location-based and preference-based search
	var searchTerms []string

	// Add destination
	searchTerms = append(searchTerms, profile.Destination)

	// Add interests as search terms
	searchTerms = append(searchTerms, profile.Interests...)

	// Add travel style
	searchTerms = append(searchTerms, profile.TravelStyle...)

	// Use your existing multi-strategy POI finding
	return p.findRelevantPOIs(ctx, strings.Join(searchTerms, " "))
}

// generatePersonalizedRecommendations creates tailored recommendations
func (p *PromptService) generatePersonalizedRecommendations(pois []*db_models.POI, profile response_models.TravelProfile, answers map[string]string) []response_models.PersonalizedRecommendation {
	var recommendations []response_models.PersonalizedRecommendation

	for _, poi := range pois {
		if len(recommendations) >= 5 { // Limit recommendations
			break
		}

		recType := p.determineRecommendationType(poi, profile)
		reason := p.generateRecommendationReason(poi, profile, answers)

		travelPOI := response_models.TravelPOI{
			ID:          poi.ID.String(),
			Name:        poi.Name,
			Description: poi.Description,
			Category:    p.categorizePOI(poi),
			Address:     poi.Address,
		}

		recommendation := response_models.PersonalizedRecommendation{
			Type:        recType,
			Title:       fmt.Sprintf("Perfect for %s lovers", strings.Join(profile.TravelStyle, " & ")),
			Description: p.generateRecommendationDescription(poi, profile),
			POI:         travelPOI,
			Reason:      reason,
		}

		recommendations = append(recommendations, recommendation)
	}

	return recommendations
}

// determineRecommendationType categorizes recommendations based on profile
func (p *PromptService) determineRecommendationType(poi *db_models.POI, profile response_models.TravelProfile) string {
	name := strings.ToLower(poi.Name)

	if strings.Contains(profile.BudgetRange, "$0-30") {
		return "budget_friendly"
	}

	if strings.Contains(name, "local") || strings.Contains(name, "traditional") {
		return "local_favorite"
	}

	if strings.Contains(name, "hidden") || strings.Contains(name, "secret") {
		return "hidden_gem"
	}

	return "must_visit"
}

// generateRecommendationReason explains why this POI matches the user's preferences
func (p *PromptService) generateRecommendationReason(poi *db_models.POI, profile response_models.TravelProfile, answers map[string]string) string {
	reasons := []string{}

	for _, style := range profile.TravelStyle {
		if strings.Contains(strings.ToLower(poi.Description), strings.ToLower(style)) {
			reasons = append(reasons, fmt.Sprintf("matches your %s travel style", style))
		}
	}

	for _, interest := range profile.Interests {
		if strings.Contains(strings.ToLower(poi.Description), strings.ToLower(interest)) {
			reasons = append(reasons, fmt.Sprintf("aligns with your interest in %s", strings.ToLower(interest)))
		}
	}

	if len(reasons) == 0 {
		return "recommended based on your destination choice"
	}

	return strings.Join(reasons, " and ")
}

// generateRecommendationDescription creates an engaging description
func (p *PromptService) generateRecommendationDescription(poi *db_models.POI, profile response_models.TravelProfile) string {
	return fmt.Sprintf("Based on your preferences for %s travel and interest in %s, this location offers the perfect experience for your %s adventure.",
		strings.Join(profile.TravelStyle, " & "),
		strings.Join(profile.Interests, ", "),
		profile.Destination)
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

// Enhanced CreateAIPlan method for narrative-style itineraries
func (p *PromptService) CreateNarrativeAIPlan(ctx context.Context, userPrompt string) (*response_models.TravelItinerary, error) {
	// Validate input
	if strings.TrimSpace(userPrompt) == "" {
		return nil, utils.ErrInvalidInput
	}

	startTime := time.Now()
	log.Printf("ts: %d - Creating narrative AI plan for prompt: %s", time.Since(startTime), userPrompt)

	// Find relevant POIs
	pois, err := p.findRelevantPOIs(ctx, userPrompt)
	if err != nil {
		return nil, utils.ErrPOINotFound
	}

	if len(pois) == 0 {
		return nil, utils.ErrPoorQualityInput
	}

	// Extract location and day count
	locations := p.ExtractLocationFromPrompt(userPrompt)
	destination := "Vietnam"
	if len(locations) > 0 {
		destination = p.formatDestination(locations[0])
	}

	dayCount := extractDayCount(userPrompt)

	// Generate enhanced AI plan
	rawResponse, err := p.generateNarrativeAIPlan(ctx, userPrompt, pois, dayCount, destination)
	if err != nil {
		log.Printf("AI generation error: %v", err)
		return nil, utils.ErrUnexpectedBehaviorOfAI
	}

	// Convert POIs to travel format
	travelPOIs := p.convertPOIsToTravelFormat(pois)

	// Build narrative itinerary
	itinerary := p.buildNarrativeItinerary(rawResponse, travelPOIs, destination, dayCount, userPrompt)

	return itinerary, nil
}

// Convert POIs to enhanced travel format
func (p *PromptService) convertPOIsToTravelFormat(pois []*db_models.POI) map[string]response_models.TravelPOI {
	travelPOIs := make(map[string]response_models.TravelPOI)

	for _, poi := range pois {
		category := p.categorizePOI(poi)
		duration := p.estimateDuration(poi, category)
		priceLevel := p.estimatePriceLevel(poi, category)
		tips := p.generatePOITips(poi, category)

		travelPOI := response_models.TravelPOI{
			ID:          poi.ID.String(),
			Name:        poi.Name,
			Description: poi.Description,
			Category:    category,
			Tags:        p.generateTravelTags(poi),
			Address:     poi.Address,
			Duration:    duration,
			PriceLevel:  priceLevel,
			Tips:        tips,
		}

		travelPOIs[poi.ID.String()] = travelPOI
	}

	return travelPOIs
}

// Categorize POI for travel context
func (p *PromptService) categorizePOI(poi *db_models.POI) string {
	name := strings.ToLower(poi.Name)
	desc := strings.ToLower(poi.Description)

	// Restaurant/Food patterns
	foodKeywords := []string{"restaurant", "cafe", "coffee", "food", "eat", "dining", "buffet", "kitchen", "nhà hàng", "quán", "cà phê"}
	for _, keyword := range foodKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			if strings.Contains(name, "cafe") || strings.Contains(name, "coffee") || strings.Contains(name, "cà phê") {
				return "Cafe"
			}
			return "Restaurant"
		}
	}

	// Accommodation patterns
	hotelKeywords := []string{"hotel", "resort", "villa", "lodge", "khách sạn", "resort"}
	for _, keyword := range hotelKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			if strings.Contains(name, "resort") || strings.Contains(name, "villa") {
				return "Resort"
			}
			return "Hotel"
		}
	}

	// Attraction patterns
	attractionKeywords := []string{"temple", "pagoda", "church", "palace", "museum", "park", "lake", "mountain", "fall", "market",
		"chùa", "đền", "bảo tàng", "công viên", "hồ", "núi", "thác", "chợ"}
	for _, keyword := range attractionKeywords {
		if strings.Contains(name, keyword) || strings.Contains(desc, keyword) {
			if strings.Contains(name, "temple") || strings.Contains(name, "pagoda") || strings.Contains(name, "church") {
				return "Religious Site"
			}
			if strings.Contains(name, "museum") || strings.Contains(name, "palace") {
				return "Cultural Site"
			}
			if strings.Contains(name, "park") || strings.Contains(name, "garden") {
				return "Park & Garden"
			}
			if strings.Contains(name, "lake") || strings.Contains(name, "mountain") || strings.Contains(name, "fall") {
				return "Natural Attraction"
			}
			if strings.Contains(name, "market") {
				return "Shopping"
			}
			return "Attraction"
		}
	}

	return "Attraction"
}

// Estimate visit duration based on POI type
func (p *PromptService) estimateDuration(poi *db_models.POI, category string) string {
	switch category {
	case "Restaurant", "Cafe":
		return "1-2 hours"
	case "Hotel", "Resort":
		return "Overnight"
	case "Shopping", "Market":
		return "1-3 hours"
	case "Museum", "Cultural Site":
		return "2-3 hours"
	case "Religious Site":
		return "30-60 minutes"
	case "Park & Garden":
		return "1-2 hours"
	case "Natural Attraction":
		return "2-4 hours"
	default:
		return "1-2 hours"
	}
}

// Estimate price level
func (p *PromptService) estimatePriceLevel(poi *db_models.POI, category string) string {
	name := strings.ToLower(poi.Name)

	// Check for luxury indicators
	luxuryKeywords := []string{"luxury", "premium", "resort", "villa", "palace", "royal", "grand"}
	for _, keyword := range luxuryKeywords {
		if strings.Contains(name, keyword) {
			return "$$$$"
		}
	}

	// Check for budget indicators
	budgetKeywords := []string{"local", "street", "market", "budget", "cheap"}
	for _, keyword := range budgetKeywords {
		if strings.Contains(name, keyword) {
			return "$"
		}
	}

	// Default by category
	switch category {
	case "Restaurant":
		return "$$"
	case "Cafe":
		return "$"
	case "Hotel":
		return "$$$"
	case "Resort":
		return "$$$$"
	case "Shopping", "Market":
		return "$"
	case "Attraction", "Cultural Site", "Religious Site":
		return "$"
	default:
		return "$$"
	}
}

// Generate travel-focused tags
func (p *PromptService) generateTravelTags(poi *db_models.POI) []string {
	var tags []string
	name := strings.ToLower(poi.Name)
	desc := strings.ToLower(poi.Description)

	// Location-based tags
	if strings.Contains(name, "da lat") || strings.Contains(name, "dalat") {
		tags = append(tags, "da-lat")
	}
	if strings.Contains(name, "saigon") || strings.Contains(name, "ho chi minh") {
		tags = append(tags, "saigon")
	}

	// Experience tags
	if strings.Contains(desc, "romantic") || strings.Contains(name, "honeymoon") {
		tags = append(tags, "romantic")
	}
	if strings.Contains(desc, "scenic") || strings.Contains(desc, "view") {
		tags = append(tags, "scenic")
	}
	if strings.Contains(desc, "local") || strings.Contains(desc, "traditional") {
		tags = append(tags, "local-favorite")
	}
	if strings.Contains(desc, "photo") || strings.Contains(desc, "instagram") {
		tags = append(tags, "instagram-worthy")
	}
	if strings.Contains(desc, "family") || strings.Contains(desc, "kid") {
		tags = append(tags, "family-friendly")
	}

	// Activity tags
	if strings.Contains(desc, "walk") || strings.Contains(desc, "hike") {
		tags = append(tags, "walking")
	}
	if strings.Contains(desc, "cultural") || strings.Contains(desc, "history") {
		tags = append(tags, "cultural")
	}
	if strings.Contains(desc, "nature") || strings.Contains(desc, "outdoor") {
		tags = append(tags, "nature")
	}

	return tags
}

// Generate helpful tips for POIs
func (p *PromptService) generatePOITips(poi *db_models.POI, category string) string {
	name := strings.ToLower(poi.Name)

	switch category {
	case "Restaurant", "Cafe":
		if strings.Contains(name, "local") || strings.Contains(name, "street") {
			return "Try the local specialties! Cash payment often preferred."
		}
		return "Consider making a reservation, especially during peak hours."
	case "Market":
		return "Bring cash and don't be afraid to negotiate prices. Best visited in the morning."
	case "Natural Attraction":
		return "Bring comfortable walking shoes and water. Early morning visits often have the best lighting."
	case "Religious Site":
		return "Dress modestly and be respectful. Remove shoes when entering temples."
	case "Cultural Site":
		return "Allow extra time to fully appreciate the exhibits. Photography rules may vary."
	default:
		return "Check opening hours before visiting."
	}
}

// Format destination name
func (p *PromptService) formatDestination(location string) string {
	location = strings.Title(strings.ToLower(location))

	// Handle specific Vietnamese locations
	switch strings.ToLower(location) {
	case "da lat", "dalat", "đà lạt":
		return "Da Lat, Vietnam"
	case "ho chi minh", "hồ chí minh", "saigon", "sài gòn":
		return "Ho Chi Minh City, Vietnam"
	case "ha noi", "hanoi", "hà nội":
		return "Hanoi, Vietnam"
	case "hoi an", "hội an":
		return "Hoi An, Vietnam"
	case "nha trang":
		return "Nha Trang, Vietnam"
	case "phu quoc", "phú quốc":
		return "Phu Quoc, Vietnam"
	default:
		return location + ", Vietnam"
	}
}

// Generate narrative AI plan with enhanced prompting
func (p *PromptService) generateNarrativeAIPlan(ctx context.Context, userPrompt string, pois []*db_models.POI, dayCount int, destination string) (string, error) {
	// Prepare POI data
	var poiList []string
	for _, poi := range pois {
		poiData := fmt.Sprintf("ID:%s|Name:%s|Category:%s|Description:%s",
			poi.ID.String(), poi.Name, p.categorizePOI(poi), poi.Description)
		poiList = append(poiList, poiData)
	}

	// Create enhanced prompt for narrative style
	prompt := p.buildNarrativePrompt(userPrompt, poiList, dayCount, destination)

	return p.aiService.GenerateStructuredPlan(ctx, prompt, poiList, dayCount)
}

// Build narrative-focused prompt
func (p *PromptService) buildNarrativePrompt(userPrompt string, pois []string, dayCount int, destination string) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Create a %d-day travel itinerary for %s in a narrative, engaging style similar to travel blogs.\n\n", dayCount, destination))

	prompt.WriteString("STYLE REQUIREMENTS:\n")
	prompt.WriteString("- Use emojis for visual appeal (🌸🌿☀️🌤️🌙)\n")
	prompt.WriteString("- Write in an enthusiastic, personal tone\n")
	prompt.WriteString("- Include practical tips and local insights\n")
	prompt.WriteString("- Group activities by time periods (Morning, Afternoon, Evening)\n")
	prompt.WriteString("- Add descriptive themes for each day\n\n")

	prompt.WriteString("Available POIs:\n")
	for _, poi := range pois {
		prompt.WriteString(fmt.Sprintf("- %s\n", poi))
	}

	prompt.WriteString(fmt.Sprintf("\nUser Request: %s\n\n", userPrompt))

	prompt.WriteString("Return a JSON structure with this format:\n")
	if dayCount > 1 {
		prompt.WriteString(`{
  "title": "Da Lat, Vietnam – 2-Day Itinerary 🌲🌸",
  "subtitle": "A breezy, romantic escape into pine forests...",
  "destination": "` + destination + `",
  "duration": "` + fmt.Sprintf("%d days", dayCount) + `",
  "travel_style": ["romantic", "nature", "cultural"],
  "overview": "Perfect for a relaxed yet memorable getaway!",
  "days": [
    {
      "day": 1,
      "title": "Arrival & Da Lat City Discovery",
      "theme": "Charming streets, French colonial vibes, and delicious local eats",
      "location": "Da Lat City Center",
      "overview": "Day summary",
      "activities": [
        {
          "title": "City Discovery & French Colonial Vibes",
          "time_block": {
            "period": "Morning",
            "start_time": "09:00",
            "end_time": "12:00",
            "description": "Charming streets and French colonial architecture"
          },
          "main_poi": {
            "id": "poi-id-from-list",
            "name": "POI Name",
            "description": "Description",
            "category": "Attraction",
            "tags": ["scenic", "cultural"]
          },
          "description": "Detailed narrative description of the activity",
          "highlights": ["Key highlight 1", "Key highlight 2"],
          "travel_tips": ["Practical tip 1", "Practical tip 2"]
        }
      ]
    }
  ]
}`)
	} else {
		prompt.WriteString(`{
  "title": "Da Lat Day Trip 🌸",
  "subtitle": "A perfect day escape...",
  "destination": "` + destination + `",
  "duration": "1 day",
  "days": [
    {
      "day": 1,
      "title": "Da Lat Highlights",
      "activities": [
        {
          "title": "Morning Discovery",
          "time_block": {
            "period": "Morning",
            "start_time": "09:00",
            "end_time": "12:00"
          },
          "main_poi": {
            "id": "poi-id",
            "name": "POI Name"
          },
          "description": "Activity description"
        }
      ]
    }
  ]
}`)
	}

	return prompt.String()
}

// Build narrative itinerary from AI response
func (p *PromptService) buildNarrativeItinerary(rawResponse string, travelPOIs map[string]response_models.TravelPOI, destination string, dayCount int, userPrompt string) *response_models.TravelItinerary {
	// Clean the AI response
	cleanedResponse := p.cleanJSONResponse(rawResponse)

	// Try to parse the AI response
	var aiItinerary struct {
		Title       string   `json:"title"`
		Subtitle    string   `json:"subtitle"`
		Destination string   `json:"destination"`
		Duration    string   `json:"duration"`
		TravelStyle []string `json:"travel_style"`
		Overview    string   `json:"overview"`
		Days        []struct {
			Day        int    `json:"day"`
			Title      string `json:"title"`
			Theme      string `json:"theme"`
			Location   string `json:"location"`
			Overview   string `json:"overview"`
			Activities []struct {
				Title     string `json:"title"`
				TimeBlock struct {
					Period      string `json:"period"`
					StartTime   string `json:"start_time"`
					EndTime     string `json:"end_time"`
					Description string `json:"description"`
				} `json:"time_block"`
				MainPOI struct {
					ID          string   `json:"id"`
					Name        string   `json:"name"`
					Description string   `json:"description"`
					Category    string   `json:"category"`
					Tags        []string `json:"tags"`
				} `json:"main_poi"`
				SupportPOIs []struct {
					ID          string   `json:"id"`
					Name        string   `json:"name"`
					Description string   `json:"description"`
					Category    string   `json:"category"`
					Tags        []string `json:"tags"`
				} `json:"support_pois"`
				Description   string   `json:"description"`
				Highlights    []string `json:"highlights"`
				TravelTips    []string `json:"travel_tips"`
				EstimatedCost string   `json:"estimated_cost"`
			} `json:"activities"`
		} `json:"days"`
	}

	// Parse the AI response
	err := json.Unmarshal([]byte(cleanedResponse), &aiItinerary)
	if err != nil {
		log.Printf("Failed to parse AI response, creating fallback itinerary: %v", err)
		return p.createFallbackNarrativeItinerary(travelPOIs, destination, dayCount, userPrompt)
	}

	// Build the final itinerary
	itinerary := &response_models.TravelItinerary{
		Title:       aiItinerary.Title,
		Subtitle:    aiItinerary.Subtitle,
		Duration:    aiItinerary.Duration,
		Destination: aiItinerary.Destination,
		TravelStyle: aiItinerary.TravelStyle,
		Overview:    aiItinerary.Overview,
		Days:        []response_models.TravelDayPlan{},
		CreatedAt:   time.Now(),
	}

	// Convert AI days to our format
	for _, aiDay := range aiItinerary.Days {
		day := response_models.TravelDayPlan{
			Day:        aiDay.Day,
			Date:       time.Now().AddDate(0, 0, aiDay.Day-1).Format("2006-01-02"),
			Title:      aiDay.Title,
			Theme:      aiDay.Theme,
			Location:   aiDay.Location,
			Overview:   aiDay.Overview,
			Activities: []response_models.TravelActivity{},
		}

		// Convert activities
		for _, aiActivity := range aiDay.Activities {
			activity := response_models.TravelActivity{
				Title: aiActivity.Title,
				TimeBlock: response_models.TimeBlock{
					Period:      aiActivity.TimeBlock.Period,
					StartTime:   aiActivity.TimeBlock.StartTime,
					EndTime:     aiActivity.TimeBlock.EndTime,
					Description: aiActivity.TimeBlock.Description,
				},
				Description:   aiActivity.Description,
				Highlights:    aiActivity.Highlights,
				TravelTips:    aiActivity.TravelTips,
				EstimatedCost: aiActivity.EstimatedCost,
			}

			// Map main POI
			if travelPOI, exists := travelPOIs[aiActivity.MainPOI.ID]; exists {
				activity.MainPOI = travelPOI
			} else {
				// Create POI from AI response data
				activity.MainPOI = response_models.TravelPOI{
					ID:          aiActivity.MainPOI.ID,
					Name:        aiActivity.MainPOI.Name,
					Description: aiActivity.MainPOI.Description,
					Category:    aiActivity.MainPOI.Category,
					Tags:        aiActivity.MainPOI.Tags,
				}
			}

			// Map support POIs
			for _, aiSupportPOI := range aiActivity.SupportPOIs {
				if travelPOI, exists := travelPOIs[aiSupportPOI.ID]; exists {
					activity.SupportPOIs = append(activity.SupportPOIs, travelPOI)
				} else {
					supportPOI := response_models.TravelPOI{
						ID:          aiSupportPOI.ID,
						Name:        aiSupportPOI.Name,
						Description: aiSupportPOI.Description,
						Category:    aiSupportPOI.Category,
						Tags:        aiSupportPOI.Tags,
					}
					activity.SupportPOIs = append(activity.SupportPOIs, supportPOI)
				}
			}

			day.Activities = append(day.Activities, activity)
		}

		itinerary.Days = append(itinerary.Days, day)
	}

	return itinerary
}

// Create fallback itinerary when AI parsing fails
func (p *PromptService) createFallbackNarrativeItinerary(travelPOIs map[string]response_models.TravelPOI, destination string, dayCount int, userPrompt string) *response_models.TravelItinerary {
	itinerary := &response_models.TravelItinerary{
		Title:       fmt.Sprintf("%s – %d-Day Itinerary 🌟", destination, dayCount),
		Subtitle:    p.generateSubtitle(destination, dayCount),
		Duration:    fmt.Sprintf("%d days", dayCount),
		Destination: destination,
		TravelStyle: p.inferTravelStyle(userPrompt),
		Overview:    p.generateOverview(destination, dayCount),
		Days:        []response_models.TravelDayPlan{},
		CreatedAt:   time.Now(),
	}

	// Convert available POIs to activities
	poiList := make([]response_models.TravelPOI, 0, len(travelPOIs))
	for _, poi := range travelPOIs {
		poiList = append(poiList, poi)
	}

	// Distribute POIs across days
	poisPerDay := len(poiList) / dayCount
	if poisPerDay == 0 {
		poisPerDay = 1
	}

	for i := 1; i <= dayCount; i++ {
		day := response_models.TravelDayPlan{
			Day:        i,
			Date:       time.Now().AddDate(0, 0, i-1).Format("2006-01-02"),
			Title:      fmt.Sprintf("Day %d Adventure", i),
			Theme:      p.generateDayTheme(i, destination),
			Location:   destination,
			Overview:   fmt.Sprintf("Explore the best of %s on day %d", destination, i),
			Activities: []response_models.TravelActivity{},
		}

		// Add activities for this day
		startIdx := (i - 1) * poisPerDay
		endIdx := startIdx + poisPerDay
		if i == dayCount {
			endIdx = len(poiList) // Include remaining POIs in last day
		}

		periods := []string{"Morning", "Afternoon", "Evening"}
		periodIdx := 0

		for j := startIdx; j < endIdx && j < len(poiList); j++ {
			poi := poiList[j]
			period := periods[periodIdx%len(periods)]

			activity := response_models.TravelActivity{
				Title: fmt.Sprintf("%s Exploration", period),
				TimeBlock: response_models.TimeBlock{
					Period:      period,
					StartTime:   fmt.Sprintf("%02d:00", 9+(periodIdx*3)),
					EndTime:     fmt.Sprintf("%02d:00", 12+(periodIdx*3)),
					Description: fmt.Sprintf("%s activities in %s", period, destination),
				},
				MainPOI:     poi,
				Description: fmt.Sprintf("Visit %s and explore the surrounding area", poi.Name),
				Highlights:  []string{poi.Name, "Local exploration", "Photo opportunities"},
				TravelTips:  []string{"Bring comfortable walking shoes", "Check opening hours"},
			}

			day.Activities = append(day.Activities, activity)
			periodIdx++
		}

		itinerary.Days = append(itinerary.Days, day)
	}

	return itinerary
}

// Clean JSON response helper
func (p *PromptService) cleanJSONResponse(response string) string {
	// Remove markdown formatting
	response = strings.ReplaceAll(response, "```json", "")
	response = strings.ReplaceAll(response, "```JSON", "")
	response = strings.ReplaceAll(response, "```", "")

	// Trim whitespace
	response = strings.TrimSpace(response)

	// Find JSON boundaries
	start := strings.Index(response, "{")
	if start == -1 {
		return response
	}

	// Find matching closing brace
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(response); i++ {
		char := response[i]

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
				return response[start : i+1]
			}
		}
	}

	return response
}

// Helper methods for generating content
func (p *PromptService) generateSubtitle(destination string, dayCount int) string {
	if strings.Contains(destination, "Da Lat") {
		return "A breezy, romantic escape into pine forests, French villas, and cool mountain air"
	}
	return fmt.Sprintf("Perfect for a %d-day memorable getaway!", dayCount)
}

func (p *PromptService) inferTravelStyle(prompt string) []string {
	lower := strings.ToLower(prompt)
	var styles []string

	if strings.Contains(lower, "romantic") || strings.Contains(lower, "couple") {
		styles = append(styles, "romantic")
	}
	if strings.Contains(lower, "nature") || strings.Contains(lower, "mountain") || strings.Contains(lower, "forest") {
		styles = append(styles, "nature")
	}
	if strings.Contains(lower, "culture") || strings.Contains(lower, "temple") || strings.Contains(lower, "museum") {
		styles = append(styles, "cultural")
	}
	if strings.Contains(lower, "adventure") || strings.Contains(lower, "hike") {
		styles = append(styles, "adventure")
	}
	if strings.Contains(lower, "food") || strings.Contains(lower, "restaurant") {
		styles = append(styles, "culinary")
	}

	if len(styles) == 0 {
		styles = []string{"leisure", "sightseeing"}
	}

	return styles
}

func (p *PromptService) generateOverview(destination string, dayCount int) string {
	return fmt.Sprintf("Perfect for a relaxed yet memorable %d-day getaway to %s!", dayCount, destination)
}

func (p *PromptService) generateDayTheme(day int, destination string) string {
	themes := []string{
		"Arrival and first impressions",
		"Deep exploration and local experiences",
		"Hidden gems and relaxation",
		"Cultural immersion and adventure",
		"Farewell and lasting memories",
	}

	if day <= len(themes) {
		return themes[day-1]
	}
	return "Continued exploration"
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
