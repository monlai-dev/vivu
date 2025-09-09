package controllers

import (
	"net/http"
	"strconv"
	"vivu/internal/models/request_models"

	"github.com/gin-gonic/gin"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type TagController struct {
	tagService services.TagServiceInterface
}

func NewTagController(tagService services.TagServiceInterface) *TagController {
	return &TagController{
		tagService: tagService,
	}
}

// ListAllTagsHandler godoc
// @Summary List all tags
// @Description Fetch a paginated list of all tags
// @Tags Tags
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(5) minimum(1) maximum(100)
// @Success 200 {array} response_models.TagResponse
// @Failure 400 {object} utils.APIResponse
// @Router /tags/list-all [get]
func (tc *TagController) ListAllTagsHandler(c *gin.Context) {
	// 1. Parse query parameters
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

	// 2. Call service layer
	tags, err := tc.tagService.GetAllTags(page, pageSize, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	// 3. Respond with success
	utils.RespondSuccess(c, tags, "Fetched tags successfully")
}

func (tc *TagController) CreateTagHandler(c *gin.Context) {
	var createTagRequest request_models.CreateTagRequest
	if err := c.ShouldBindJSON(&createTagRequest); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Call service layer to insert tag
	err := tc.tagService.InsertTagTx(createTagRequest, c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	// Respond with success
	utils.RespondSuccess(c, nil, "Tag created successfully")
}
