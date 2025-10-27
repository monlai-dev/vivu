package controllers

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"vivu/internal/models/request_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type POIsController struct {
	poiService services.POIServiceInterface
}

func NewPOIsController(poiService services.POIServiceInterface) *POIsController {
	return &POIsController{
		poiService: poiService,
	}
}

// GetPoiById godoc
// @Summary Get POI by ID
// @Description Fetch a Point of Interest (POI) by its ID
// @Tags POIs
// @Param id path string true "POI ID"
// @Success 200 {object} response_models.POI
// @Failure 404 {object} utils.APIResponse
// @Router /pois/pois-details/{id} [get]
func (p *POIsController) GetPoiById(c *gin.Context) {
	poiId := c.Param("id")
	if poiId == "" {
		utils.RespondError(c, http.StatusBadRequest, "POI ID is required")
		return
	}

	poi, err := p.poiService.GetPOIById(poiId, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, poi, "POI fetched successfully")
}

// GetPoisByProvince godoc
// @Summary Get POIs by Province
// @Description Fetch a list of POIs by province ID with pagination
// @Tags POIs
// @Param provinceId path string true "Province ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(5) minimum(1) maximum(100)
// @Success 200 {array} response_models.POI
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Router /pois/provinces/{provinceId} [get]
func (p *POIsController) GetPoisByProvince(c *gin.Context) {
	provinceId := c.Param("provinceId")
	if provinceId == "" {
		utils.RespondError(c, http.StatusBadRequest, "Province ID is required")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "5")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page size (must be 1-100)")
		return
	}

	pois, err := p.poiService.GetPoisByProvince(provinceId, page, pageSize, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, pois, "POIs fetched successfully")
}

// CreatePoi godoc
// @Summary Create a new POI
// @Description Create a new Point of Interest (POI)
// @Tags POIs
// @Accept json
// @Produce json
// @Param request body request_models.CreatePoiRequest true "POI creation payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /pois/create-poi [post]
func (p *POIsController) CreatePoi(c *gin.Context) {
	var req request_models.CreatePoiRequest
	if err := c.ShouldBindJSON(&req); err != nil {

		utils.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := context.Background()

	if err := p.poiService.CreatePois(req, ctx); err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI created successfully")
}

// DeletePoi godoc
// @Summary Delete a POI
// @Description Delete a Point of Interest (POI) by its ID
// @Tags POIs
// @Accept json
// @Produce json
// @Param request body request_models.DeletePoiRequest true "POI deletion payload"
// @Success 200 {object} utils.APIResponse
// @Router /pois/delete-poi [delete]
// @Security BearerAuth
func (p *POIsController) DeletePoi(c *gin.Context) {

	var deleteRequest request_models.DeletePoiRequest
	if err := c.ShouldBindJSON(&deleteRequest); err != nil {

		utils.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.poiService.DeletePoi(deleteRequest.ID, c.Request.Context()); err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI deleted successfully")
}

// UpdatePoi godoc
// @Summary Update a POI
// @Description Update a Point of Interest (POI) by its ID
// @Tags POIs
// @Accept json
// @Produce json
// @Param request body request_models.UpdatePoiRequest true "POI update payload"
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Router /pois/update-poi [put]
func (p *POIsController) UpdatePoi(c *gin.Context) {
	var updateRequest request_models.UpdatePoiRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		utils.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.poiService.UpdatePoi(updateRequest, c.Request.Context()); err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI updated successfully")
}

// ListPois godoc
// @Summary List POIs with pagination
// @Description Fetch a paginated list of Points of Interest (POIs)
// @Tags POIs
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(5) minimum(1) maximum(100)
// @Success 200 {array} response_models.POI
// @Router /pois/list-pois [get]
func (p *POIsController) ListPois(c *gin.Context) {

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "5")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page size (must be 1-100)")
		return
	}

	pois, err := p.poiService.ListPois(context.Background(), page, pageSize)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, pois, "POIs fetched successfully")
}

// SearchPoiByNameAndProvince godoc
// @Summary Search POIs by name and province
// @Description Search for Points of Interest (POIs) by name and province ID with pagination
// @Tags POIs
// @Param name query string true "POI name"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(5) minimum(1) maximum(100)
// @Success 200 {array} response_models.POI
// @Failure 400 {object} utils.APIResponse
// @Router /pois/search-poi-by-name-and-province [get]
func (p *POIsController) SearchPoiByNameAndProvince(c *gin.Context) {
	name := c.Query("name")
	//provinceId := c.Query("provinceId")

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "5")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page size (must be 1-100)")
		return
	}

	pois, err := p.poiService.SearchPoiByNameAndProvince(name, "", page, pageSize, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, pois, "POIs fetched successfully")
}
