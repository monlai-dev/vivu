package services

import (
	"context"
	"encoding/json"
	"fmt"
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

func (p *PromptService) CreatePrompt(ctx context.Context, prompt string) (string, error) {
	//vector, err := p.aiService.GetEmbedding(ctx, prompt)
	//if err != nil {
	//	return "", err
	//}
	//
	//poiseEmbeddedIds, err := p.embededRepo.GetListOfPoiEmbededByVector(vector, nil)
	//{
	//	var pisIdList []string
	//
	//	if err != nil {
	//		return "", err
	//	}
	//
	//	if poiseEmbeddedIds != nil {
	//		for _, pisEmbedded := range poiseEmbeddedIds {
	//
	//			pisIdList = append(pisIdList, pisEmbedded.PoiID)
	//		}
	//
	//		pois, err := p.poisRepo.ListPoisByPoisId(ctx, pisIdList)
	//
	//	}
	//
	//}
	panic("")
}

func (p *PromptService) PromptInput(ctx context.Context, request request_models.CreateTagRequest) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PromptService) CreateAIPlan(ctx context.Context, userPrompt string) ([]response_models.ActivityPlanBlock, error) {
	embedding, err := p.aiService.GetEmbedding(ctx, userPrompt)
	if err != nil {
		return nil, err
	}

	embeddedPois, err := p.embededRepo.GetListOfPoiEmbededByVector(embedding, nil)
	if err != nil || len(embeddedPois) == 0 {
		return nil, fmt.Errorf("no POIs found")
	}

	var poiIDs []string
	for _, ep := range embeddedPois {
		poiIDs = append(poiIDs, ep.PoiID)
	}

	pois, err := p.poisRepo.ListPoisByPoisId(ctx, poiIDs)
	if err != nil {
		return nil, err
	}

	var poiTextList []string
	poiMap := make(map[string]response_models.ActivityPOI)
	for _, poi := range pois {
		poiTextList = append(poiTextList, fmt.Sprintf("%p (ID: %p): %p", poi.Name, poi.ID, poi.Description))
		poiMap[poi.ID.String()] = response_models.ActivityPOI{
			ID:          poi.ID.String(),
			Name:        poi.Name,
			Description: poi.Description,
			ProvinceID:  poi.ProvinceID.String(),
			CategoryID:  poi.CategoryID.String(),
			Tags:        flattenTags(poi.Tags),
		}
	}

	rawJSON, err := p.aiService.GenerateStructuredPlan(ctx, userPrompt, poiTextList)
	if err != nil {
		return nil, err
	}

	var skeleton []struct {
		Activity          string   `json:"activity"`
		StartTime         string   `json:"start_time"`
		EndTime           string   `json:"end_time"`
		MainPoiID         string   `json:"main_poi_id"`
		AlternativePoiIDs []string `json:"alternative_poi_ids"`
		WhatToDo          string   `json:"what_to_do"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &skeleton); err != nil {
		return nil, fmt.Errorf("invalid OpenAI plan JSON: %w", err)
	}

	var finalPlan []response_models.ActivityPlanBlock
	for _, block := range skeleton {
		mainPOI, ok := poiMap[block.MainPoiID]
		if !ok {
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

	return finalPlan, nil
}

func flattenTags(tags []*db_models.Tag) []string {
	var out []string
	for _, tag := range tags {
		// Combine both language names (e.g., "waterfall/thác nước")
		out = append(out, fmt.Sprintf("%s/%s", tag.EnName, tag.ViName))
	}
	return out
}
