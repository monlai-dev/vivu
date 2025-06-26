package controllers

import (
	"net/http"
	"strconv"

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

func (tc *TagController) ListAllTagsHandler(c *gin.Context) {
	// 1. Parse query parameters
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")

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
