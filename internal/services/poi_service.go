package services

import (
	"context"
	"github.com/google/uuid"
	"log"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type POIServiceInterface interface {
	GetPOIById(id string, ctx context.Context) (response_models.POI, error)
	GetPoisByProvince(province string, page, pageSize int, ctx context.Context) ([]response_models.POI, error)
	CreatePois(pois request_models.CreatePoiRequest, ctx context.Context) error
}

type PoiService struct {
	poiRepository repositories.POIRepository
}

func (p *PoiService) CreatePois(pois request_models.CreatePoiRequest, ctx context.Context) error {

	newPOI := &db_models.POI{
		Name:         pois.Name,
		Latitude:     pois.Latitude,
		Longitude:    pois.Longitude,
		ProvinceID:   pois.Province,
		CategoryID:   pois.Category,
		OpeningHours: pois.OpeningHours,
		ContactInfo:  pois.ContactInfo,
		Address:      pois.Address,
	}

	if pois.PoiDetails != nil {
		newPOI.Description = pois.PoiDetails.Description
		newPOI.Details = db_models.POIDetail{
			Images: pois.PoiDetails.Image,
		}
	}

	if _, err := p.poiRepository.CreatePoi(ctx, newPOI); err != nil {
		log.Printf("Error creating POI: %v", err)

		return utils.ErrDatabaseError
	}

	return nil
}

func (p *PoiService) GetPOIById(id string, ctx context.Context) (response_models.POI, error) {
	poi, err := p.poiRepository.GetByIDWithDetails(ctx, id)
	if err != nil {
		return response_models.POI{}, utils.ErrDatabaseError
	}

	if poi == nil {
		return response_models.POI{}, utils.ErrPOINotFound
	}

	var poiDetails *response_models.PoiDetails
	if poi.Details.ID != uuid.Nil {
		poiDetails = &response_models.PoiDetails{
			ID:          poi.Details.ID.String(),
			Description: poi.Description, // or poi.Details.Description if preferred
			Image:       poi.Details.Images,
		}
	}

	return response_models.POI{
		ID:           poi.ID.String(),
		Name:         poi.Name,
		Latitude:     poi.Latitude,
		Longitude:    poi.Longitude,
		Category:     poi.Category.Name,
		OpeningHours: poi.OpeningHours,
		ContactInfo:  poi.ContactInfo,
		Address:      poi.Address,
		PoiDetails:   poiDetails,
	}, nil
}

func (p *PoiService) GetPoisByProvince(province string, page, pageSize int, ctx context.Context) ([]response_models.POI, error) {

	pois, err := p.poiRepository.ListPoisByProvinceId(ctx, province, page, pageSize)
	if err != nil {
		return nil, utils.ErrDatabaseError
	}

	if len(pois) == 0 {
		return []response_models.POI{}, utils.ErrPOINotFound
	}

	poiResponses := make([]response_models.POI, 0, len(pois))

	//SetupPoisIndex()
	//BulkIndexPOIs(ctx, "poi_v1", pois)

	for _, poi := range pois {
		var poiDetails *response_models.PoiDetails
		if poi.Details.ID != uuid.Nil {

			poiDetails = &response_models.PoiDetails{
				ID:          poi.Details.ID.String(),
				Description: poi.Description,
				Image:       poi.Details.Images,
			}
		}

		poiResponses = append(poiResponses, response_models.POI{
			ID:           poi.ID.String(),
			Name:         poi.Name,
			Latitude:     poi.Latitude,
			Longitude:    poi.Longitude,
			Category:     poi.Category.Name,
			OpeningHours: poi.OpeningHours,
			ContactInfo:  poi.ContactInfo,
			Address:      poi.Address,
			PoiDetails:   poiDetails,
		})
	}

	return poiResponses, nil
}

//func ExportPOIsToExcel(db *gorm.DB, filename string) error {
//	// 1) Load data
//	var pois []*db_models.POI
//	if err := db.
//		Preload("Province").
//		Preload("Category").
//		Find(&pois).Error; err != nil {
//		return fmt.Errorf("query POIs: %w", err)
//	}
//
//	// 2) Create workbook
//	f := excelize.NewFile()
//	const sheet = "POIs"
//	// Ensure our sheet exists (NewFile has default "Sheet1")
//	// Rename default to our name for neatness:
//	f.SetSheetName("Sheet1", sheet)
//
//	// 3) Header row
//	headers := []string{
//		"ID",
//		"Name",
//		"Latitude",
//		"Longitude",
//		"Province",
//		"Category",
//		"Status",
//		"OpeningHours",
//		"ContactInfo",
//		"Description",
//		"Address",
//		"CreatedAt",
//		"UpdatedAt",
//	}
//	for i, h := range headers {
//		col, _ := excelize.ColumnNumberToName(i + 1) // 1->A, 2->B...
//		cell := fmt.Sprintf("%s1", col)
//		if err := f.SetCellValue(sheet, cell, h); err != nil {
//			return err
//		}
//	}
//
//	// 4) Styles
//	headerStyle, _ := f.NewStyle(&excelize.Style{
//		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
//		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#4F81BD"}},
//		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
//		Border: []excelize.Border{
//			{Type: "bottom", Color: "FFFFFF", Style: 1},
//		},
//	})
//	_ = f.SetCellStyle(sheet, "A1", fmt.Sprintf("%s1", mustColName(len(headers))), headerStyle)
//
//	latLongStyle, _ := f.NewStyle(&excelize.Style{
//		NumFmt: 2, // "#,##0.00"
//		Alignment: &excelize.Alignment{
//			Horizontal: "left",
//			Vertical:   "center",
//		},
//	})
//
//	dateStyle, _ := f.NewStyle(&excelize.Style{
//		NumFmt: 22, // "m/d/yy h:mm"
//		Alignment: &excelize.Alignment{
//			Horizontal: "left",
//			Vertical:   "center",
//		},
//	})
//
//	// Freeze top row for readability
//	_ = f.SetPanes(sheet, &excelize.Panes{
//		Freeze:      true,
//		Split:       true,
//		XSplit:      0,
//		YSplit:      1,
//		TopLeftCell: "A2",
//		ActivePane:  "bottomLeft",
//	})
//
//	// 5) Write data rows
//	for i, p := range pois {
//		r := i + 2 // data starts at row 2
//		set := func(col string, v any) {
//			if err := f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r), v); err != nil {
//				log.Printf("set %s%d: %v", col, r, err)
//			}
//		}
//
//		set("A", p.ID.String())
//		set("B", p.Name)
//		set("C", p.Latitude)
//		set("D", p.Longitude)
//		set("E", safeName(p.Province.Name))
//		set("F", safeName(p.Category.Name))
//		set("G", p.Status)
//		set("H", p.OpeningHours)
//		set("I", p.ContactInfo)
//		set("J", p.Description)
//		set("K", p.Address)
//		set("L", strconv.Itoa(int(p.CreatedAt))) // apply date style later
//		set("M", p.DeletedAt.Time.String())
//	}
//
//	// Apply numeric styling for lat/long columns (C & D)
//	if len(pois) > 0 {
//		lastRow := len(pois) + 2 - 1
//		_ = f.SetCellStyle(sheet, "C2", fmt.Sprintf("C%d", lastRow), latLongStyle)
//		_ = f.SetCellStyle(sheet, "D2", fmt.Sprintf("D%d", lastRow), latLongStyle)
//		_ = f.SetCellStyle(sheet, "L2", fmt.Sprintf("L%d", lastRow), dateStyle)
//		_ = f.SetCellStyle(sheet, "M2", fmt.Sprintf("M%d", lastRow), dateStyle)
//	}
//
//	// 6) Column widths (tweak as you like)
//	_ = f.SetColWidth(sheet, "A", "A", 38) // ID
//	_ = f.SetColWidth(sheet, "B", "B", 28) // Name
//	_ = f.SetColWidth(sheet, "C", "D", 12) // Lat/Long
//	_ = f.SetColWidth(sheet, "E", "G", 18) // Province/Category/Status
//	_ = f.SetColWidth(sheet, "H", "I", 24) // Opening/Contact
//	_ = f.SetColWidth(sheet, "J", "J", 50) // Description
//	_ = f.SetColWidth(sheet, "K", "K", 40) // Address
//	_ = f.SetColWidth(sheet, "L", "M", 20) // Dates
//
//	_ = f.SetColVisible(sheet, "A", false) // hide ID column
//
//	// 7) Save
//	if err := f.SaveAs(filename); err != nil {
//		return fmt.Errorf("save excel: %w", err)
//	}
//
//	log.Println("Exported POIs to", filename)
//	return nil
//}

//func mustColName(n int) string {
//	s, _ := excelize.ColumnNumberToName(n)
//	return s
//}
//
//func safeName(s string) string {
//	if s == "" {
//		return ""
//	}
//	return s
//}

func NewPOIService(poiRepository repositories.POIRepository) POIServiceInterface {
	return &PoiService{
		poiRepository: poiRepository,
	}
}
