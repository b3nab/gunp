package app

import (
	"errors"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
)

func updateWidthColumns(t table.Model, w int) []table.Column {
	columns := t.Columns()
	rows := t.Rows()
	takenWidth := 0
	for i := range columns {
		if i == 0 {
			columns[i].Width = 5
			continue
		}
		// default maxWidth to m.width / number of columns
		maxWidth := (w - takenWidth) / len(columns)
		for _, r := range rows {
			minWidthRow := len(r[i])
			if minWidthRow > maxWidth {
				maxWidth = minWidthRow
			}

			if len(columns[i].Title) > maxWidth {
				maxWidth = len(columns[i].Title)
			}

			if maxWidth > (w / len(columns)) {
				maxWidth = (w / len(columns)) - 2
			}
		}
		takenWidth += maxWidth
		columns[i].Width = maxWidth
	}
	return columns
}

func getSelectedRow(t table.Model) (int, error) {
	selectedStrIndex := t.SelectedRow()
	if selectedStrIndex != nil {
		if selectedIndex, err := strconv.Atoi(selectedStrIndex[0]); err == nil {
			return selectedIndex, nil
		}
	}
	return -1, errors.New("row not found")
}
