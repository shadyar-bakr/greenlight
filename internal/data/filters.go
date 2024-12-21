package data

import "github.com/shadyar-bakr/greenlight/internal/validator"

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a reasonable number")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must not be more than 100")
	v.Check(validator.PermittedValue(f.SortBy, f.SortSafeList...), "sort", "invalid sort value")
}

type Filters struct {
	Page         int
	PageSize     int
	SortBy       string
	SortSafeList []string
}
