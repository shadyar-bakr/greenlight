package data

import (
	"strings"

	"github.com/shadyar-bakr/greenlight/internal/validator"
)

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a reasonable number")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must not be more than 100")
	v.Check(validator.PermittedValue(f.SortBy, f.SortSafeList...), "sort", "invalid sort value")
}

type Filters struct {
	Page         int      `json:"page"`
	PageSize     int      `json:"page_size"`
	SortBy       string   `json:"sort"`
	SortSafeList []string `json:"-"`
}

func (f Filters) GetSortColumn() string {
	if f.SortBy == "" {
		return "id"
	}

	sortColumn := strings.TrimPrefix(f.SortBy, "-")

	for _, safeValue := range f.SortSafeList {
		if sortColumn == strings.TrimPrefix(safeValue, "-") {
			return sortColumn
		}
	}

	return "id"
}

func (f Filters) GetSortDirection() string {
	if strings.HasPrefix(f.SortBy, "-") {
		return "DESC"
	}
	return "ASC"
}

func (f Filters) Getlimit() int {
	return f.PageSize
}

func (f Filters) Getoffset() int {
	return (f.Page - 1) * f.PageSize
}
