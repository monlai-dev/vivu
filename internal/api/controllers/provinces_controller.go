package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type ProvincesController struct {
	provinceService services.ProvinceServiceInterface
}

func NewProvincesController(provinceService services.ProvinceServiceInterface) *ProvincesController {
	return &ProvincesController{
		provinceService: provinceService,
	}
}

// GetAllProvinces godoc
// @Summary Get all provinces
// @Description Fetch a paginated list of provinces
// @Tags Provinces
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param pageSize query int false "Page size (default: 5, max: 100)"
// @Success 200 {object} response_models.ProvinceResponse
// @Failure 400 {object} utils.APIResponse
// @Security BearerAuth
// @Router /provinces/list-all [get]
func (p *ProvincesController) GetAllProvinces(c *gin.Context) {

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

	pois, err := p.provinceService.GetAllTags(page, pageSize, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, pois, "Provinces fetched successfully")
}

// FindProvincesByName godoc
// @Summary Find province by name
// @Description Fetch province details by its name
// @Tags Provinces
// @Accept json
// @Produce json
// @Param province_name path string true "Province Name"
// @Success 200 {object} response_models.ProvinceResponse
// @Failure 400 {object} utils.APIResponse
// @Security BearerAuth
// @Router /provinces/find-by-name/{province_name} [get]
func (p *ProvincesController) FindProvincesByName(c *gin.Context) {
	id := c.Param("province_name")
	if id == "" {
		utils.RespondError(c, http.StatusBadRequest, "Province ID is required")
		return
	}

	province, err := p.provinceService.FindProvincesByName(id, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, province, "Province fetched successfully")
}
