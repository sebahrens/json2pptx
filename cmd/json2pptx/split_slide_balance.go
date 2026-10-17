package main

import "sort"

// balancedTableChunks treats group_size as a ceiling. Boundaries follow the
// balanced cumulative row targets, moving to the nearest legal merged-group
// boundary that leaves enough capacity for the remaining pages.
func balancedTableChunks(rows [][]TableCellInput, ceiling int) [][][]TableCellInput {
	if len(rows) == 0 || ceiling <= 0 {
		return nil
	}
	if ceiling >= len(rows) {
		return [][][]TableCellInput{rows}
	}
	parts := (len(rows) + ceiling - 1) / ceiling
	boundaries := tableRowGroupBoundaries(rows)
	// Greedy farthest legal packing gives the minimum number of pages needed
	// by each suffix. It lets the balance pass avoid a locally close boundary
	// that strands a merged group on an extra page.
	suffixPages := make([]int, len(boundaries))
	for i := len(boundaries) - 2; i >= 0; i-- {
		next := sort.SearchInts(boundaries, boundaries[i]+ceiling+1) - 1
		if next <= i {
			return nil
		}
		suffixPages[i] = 1 + suffixPages[next]
	}
	base, extra := len(rows)/parts, len(rows)%parts
	start := 0
	chunks := make([][][]TableCellInput, 0, parts)
	for page := 1; page <= parts; page++ {
		remaining := parts - page
		high := min(len(boundaries)-1-remaining, sort.SearchInts(boundaries, boundaries[start]+ceiling+1)-1)
		low := start + 1
		if high < low {
			return nil
		}
		low += sort.Search(high-low+1, func(i int) bool { return suffixPages[low+i] <= remaining })
		if low > high {
			return nil
		}
		target := page*base + min(page, extra)
		chosen := low + sort.Search(high-low+1, func(i int) bool { return boundaries[low+i] >= target })
		if chosen > high {
			chosen = high
		}
		if chosen > low && absInt(boundaries[chosen-1]-target) < absInt(boundaries[chosen]-target) {
			chosen--
		}
		chunks = append(chunks, rows[boundaries[start]:boundaries[chosen]])
		start = chosen
	}
	return chunks
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// A boundary is legal once every row_span that started above it has ended.
func tableRowGroupBoundaries(rows [][]TableCellInput) []int {
	boundaries := []int{0}
	reach := 0
	for i, row := range rows {
		for _, cell := range row {
			if cell.RowSpan > 1 {
				reach = max(reach, min(len(rows), i+cell.RowSpan))
			}
		}
		if i+1 >= reach {
			boundaries = append(boundaries, i+1)
		}
	}
	return boundaries
}

func tableBoundaryCrossesSpan(rows [][]TableCellInput, boundary int) bool {
	for i, row := range rows[:boundary] {
		for _, cell := range row {
			if cell.RowSpan > 1 && i+cell.RowSpan > boundary {
				return true
			}
		}
	}
	return false
}
