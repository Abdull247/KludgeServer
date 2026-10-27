package catalog

func paginate(items []Item, page, perPage int) ItemPage {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 12
	}
	if perPage > 48 {
		perPage = 48
	}

	total := len(items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	var next *int
	var prev *int
	if page < totalPages && total > 0 {
		n := page + 1
		next = &n
	}
	if page > 1 {
		p := page - 1
		prev = &p
	}

	slice := items[start:end]
	if slice == nil {
		slice = []Item{}
	}

	return ItemPage{
		Items: slice,
		Pagination: Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    next != nil,
			HasPrev:    prev != nil,
			NextPage:   next,
			PrevPage:   prev,
		},
	}
}

func railFrom(id, slug, title, subtitle, algorithm, href string, items []Item, page, perPage int) Rail {
	p := paginate(items, page, perPage)
	return Rail{
		ID:         id,
		Slug:       slug,
		Title:      title,
		Subtitle:   subtitle,
		Algorithm:  algorithm,
		Href:       href,
		Items:      p.Items,
		Pagination: p.Pagination,
	}
}
