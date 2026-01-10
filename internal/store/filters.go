package store

import "errors"

type Filter struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func (f *Filter) Limit() int {
	return f.PageSize
}

func (f *Filter) Offset() int {
	return (f.Page - 1) * f.PageSize
}

func (f *Filter) Validate() error {
	if f.Page < 0 || f.Page > 10_000_000 {
		return errors.New("page must be between 1 and ten million")
	}
	if f.PageSize < 0 || f.PageSize > 1000 {
		return errors.New("page size must be between 1 and 1000")
	}

	err := f.validateSort()
	if err != nil {
		return err
	}

	return nil
}

func (f *Filter) validateSort() error {
	sortVal := f.Sort

	if sortVal == "" {
		return nil
	}

	if string(sortVal[0]) == "-" {
		sortVal = f.Sort[1:]
	}

	for _, key := range f.SortSafeList {
		if sortVal == key {
			return nil
		}
	}
	return errors.New("invalid sort field")
}

type Metadata struct {
	CurrentPage  int `json:"current_page"`
	PageSize     int `json:"page_size"`
	FirstPage    int `json:"first_page"`
	LastPage     int `json:"last_page"`
	TotalRecords int `json:"total_records"`
}

func CalculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     (totalRecords + pageSize - 1) / pageSize,
		TotalRecords: totalRecords,
	}
}