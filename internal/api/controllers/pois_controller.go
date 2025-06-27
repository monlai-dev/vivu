package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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
